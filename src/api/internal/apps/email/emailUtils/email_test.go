package emailUtils

import "testing"

func TestNormalizeAddress(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  string
		valid bool
	}{
		{input: "Recipient@Example.COM", want: "recipient@example.com", valid: true},
		{input: " recipient@example.com ", want: "recipient@example.com", valid: true},
		{input: "recipient+tag@example.com", want: "recipient+tag@example.com", valid: true},
		{input: ""},
		{input: "invalid"},
		{input: "姓名 <recipient@example.com>"},
		{input: "recipient@example.com\r\nBcc: other@example.com"},
	} {
		address, valid := NormalizeAddress(test.input)
		if address != test.want || valid != test.valid {
			t.Fatalf("邮箱规范化结果错误：%q %t", address, valid)
		}
	}
}
