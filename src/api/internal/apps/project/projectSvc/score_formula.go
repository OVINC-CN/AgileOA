package projectSvc

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/OVINC-CN/AgileOA/src/api/internal/config"
)

type scoreFormulaParser struct {
	source   string
	position int
	nodes    int
	depth    int
	allowed  map[string]bool
	scores   map[string]*float64
}

func scoreFormula(source string, names []string, scores map[string]*float64) (float64, error) {
	// 使用受限语法解析，不执行脚本、任意函数、维度条件或其他字段。
	if strings.TrimSpace(source) == "" ||
		!validText(source, config.Config.Project.ScoreFormulaMaxLength, true) {
		return 0, errors.New("公式不能为空或超过长度限制")
	}
	parser := scoreFormulaParser{
		source: source, allowed: make(map[string]bool, len(names)), scores: scores,
	}
	for _, name := range names {
		parser.allowed[name] = true
	}
	result, err := parser.expression()
	if err != nil {
		return 0, err
	}
	parser.spaces()
	if parser.position != len(source) {
		return 0, errors.New("公式包含不支持的内容，仅支持子项分数、四则运算、MAX 和 MIN")
	}
	return result, nil
}

func (p *scoreFormulaParser) expression() (float64, error) {
	left, err := p.product()
	for err == nil {
		p.spaces()
		if p.peek() != '+' && p.peek() != '-' {
			break
		}
		operation := p.peek()
		p.position++
		var right float64
		right, err = p.product()
		if operation == '+' {
			left += right
		} else {
			left -= right
		}
		if err == nil {
			err = p.checkResult(left)
		}
	}
	return left, err
}

func (p *scoreFormulaParser) product() (float64, error) {
	left, err := p.primary()
	for err == nil {
		p.spaces()
		if p.peek() != '*' && p.peek() != '/' {
			break
		}
		operation := p.peek()
		p.position++
		var right float64
		right, err = p.primary()
		if err == nil && operation == '/' && right == 0 && p.scores != nil {
			return 0, errors.New("除数不能为 0")
		}
		if err != nil {
			break
		}
		switch {
		case operation == '*':
			left *= right
		case right != 0:
			left /= right
		}
		err = p.checkResult(left)
	}
	return left, err
}

func (p *scoreFormulaParser) primary() (float64, error) {
	p.nodes++
	p.depth++
	defer func() { p.depth-- }()
	if p.nodes > config.Config.Project.ScoreFormulaMaxNodes ||
		p.depth > config.Config.Project.ScoreFormulaMaxDepth {
		return 0, errors.New("公式超过计算节点或嵌套层数限制")
	}
	p.spaces()
	switch p.peek() {
	case '+', '-':
		negative := p.peek() == '-'
		p.position++
		value, err := p.primary()
		if negative {
			value = -value
		}
		return value, err
	case '(':
		p.position++
		value, err := p.expression()
		if err != nil {
			return 0, err
		}
		return value, p.expect(')')
	case '[':
		return p.reference()
	default:
		if p.peek() >= '0' && p.peek() <= '9' || p.peek() == '.' {
			return p.number()
		}
		return p.function()
	}
}

func (p *scoreFormulaParser) reference() (float64, error) {
	p.position++
	var name strings.Builder
	closed := false
	for p.position < len(p.source) {
		char := p.source[p.position]
		p.position++
		if char == ']' {
			closed = true
			break
		}
		if char == '\\' {
			if p.peek() != '\\' && p.peek() != ']' {
				return 0, errors.New("子项引用转义无效")
			}
			char = p.peek()
			p.position++
		}
		name.WriteByte(char)
	}
	if !closed || !p.allowed[name.String()] {
		return 0, errors.New("公式只能引用当前项目已配置的子项，请使用插入子项")
	}
	if p.scores == nil {
		return 0, nil
	}
	value := p.scores[name.String()]
	if value == nil {
		return 0, nil
	}
	return *value, p.checkResult(*value)
}

func (p *scoreFormulaParser) number() (float64, error) {
	start := p.position
	for p.peek() >= '0' && p.peek() <= '9' {
		p.position++
	}
	if p.peek() == '.' {
		p.position++
		for p.peek() >= '0' && p.peek() <= '9' {
			p.position++
		}
	}
	value, err := strconv.ParseFloat(p.source[start:p.position], 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, errors.New("公式数字无效或超出支持范围")
	}
	return value, nil
}

func (p *scoreFormulaParser) function() (float64, error) {
	start := p.position
	for p.peek() >= 'a' && p.peek() <= 'z' || p.peek() >= 'A' && p.peek() <= 'Z' {
		p.position++
	}
	name := strings.ToUpper(p.source[start:p.position])
	if name != "MAX" && name != "MIN" {
		return 0, errors.New("公式只支持 MAX、MIN，条件须在公式框外配置")
	}
	if err := p.expect('('); err != nil {
		return 0, err
	}
	value, err := p.expression()
	count := 1
	for err == nil {
		p.spaces()
		if p.peek() != ',' {
			break
		}
		p.position++
		var next float64
		next, err = p.expression()
		if name == "MAX" {
			value = math.Max(value, next)
		} else {
			value = math.Min(value, next)
		}
		count++
	}
	if err != nil {
		return 0, err
	}
	if count < 2 {
		return 0, errors.New("MAX、MIN 至少需要两个数值参数")
	}
	return value, p.expect(')')
}

func (p *scoreFormulaParser) expect(char byte) error {
	p.spaces()
	if p.peek() != char {
		return fmt.Errorf("公式缺少 %c", char)
	}
	p.position++
	return nil
}

func (p *scoreFormulaParser) spaces() {
	for p.position < len(p.source) {
		char, size := utf8.DecodeRuneInString(p.source[p.position:])
		if !unicode.IsSpace(char) {
			break
		}
		p.position += size
	}
}

func (p *scoreFormulaParser) peek() byte {
	if p.position >= len(p.source) {
		return 0
	}
	return p.source[p.position]
}

func (p *scoreFormulaParser) checkResult(value float64) error {
	if p.scores != nil && (math.IsNaN(value) || math.IsInf(value, 0)) {
		return errors.New("计算结果超出支持范围")
	}
	return nil
}
