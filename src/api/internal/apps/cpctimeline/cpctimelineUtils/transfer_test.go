package cpctimelineUtils

import (
	"testing"

	"github.com/OVINC-CN/AgileOA/src/api/internal/apps/cpctimeline/cpctimelineModels"
)

func transferFixtureNodes() []cpctimelineModels.Node {
	return []cpctimelineModels.Node{
		{ID: "node_first", Fields: []cpctimelineModels.Field{
			{ID: "date", Type: "date"}, {ID: "note", Type: "text"},
			{ID: "birth", Type: "date", StudentField: "birth_date"},
		}},
		{ID: "node_second", Fields: []cpctimelineModels.Field{
			{ID: "date", Type: "date"}, {ID: "note", Type: "text"},
		}},
	}
}

func transferFixtureMapping() cpctimelineModels.Transfer {
	return cpctimelineModels.Transfer{TargetTemplateID: 2,
		NodeMappings: []cpctimelineModels.TransferNodeMapping{
			{
				SourceNodeID: "node_first",
				TargetNodeID: "node_first",
				Fields: []cpctimelineModels.TransferFieldMapping{
					{SourceFieldID: "date", TargetFieldID: "date"},
				},
			},
			{
				SourceNodeID: "node_first",
				TargetNodeID: "node_second",
				Fields: []cpctimelineModels.TransferFieldMapping{
					{SourceFieldID: "date", TargetFieldID: "date"},
				},
			},
			{
				SourceNodeID: "node_second",
				TargetNodeID: "node_first",
				Fields: []cpctimelineModels.TransferFieldMapping{
					{SourceFieldID: "note", TargetFieldID: "note"},
				},
			},
			{SourceNodeID: "node_second", TargetNodeID: "node_second",
				Fields: []cpctimelineModels.TransferFieldMapping{}},
		},
	}
}

func TestTransferAllowsManyToManyAndAttachmentOnlyMappings(t *testing.T) {
	if err := ValidateTransfer(
		transferFixtureNodes(),
		transferFixtureNodes(),
		transferFixtureMapping(),
	); err != nil {
		t.Fatal(err)
	}
}

func TestTransferRejectsOverwritesReferencesAndStaleFields(t *testing.T) {
	cases := []struct {
		name   string
		change func(*cpctimelineModels.Transfer)
	}{
		{"duplicate_pair", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[1].TargetNodeID = "node_first"
		}},
		{"target_overwrite", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[2].Fields = []cpctimelineModels.TransferFieldMapping{
				{SourceFieldID: "date", TargetFieldID: "date"},
			}
		}},
		{"source_reference", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[0].Fields[0].SourceFieldID = "birth"
		}},
		{"target_reference", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[0].Fields[0].TargetFieldID = "birth"
		}},
		{"type_mismatch", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[0].Fields[0].TargetFieldID = "note"
		}},
		{"missing_node", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[0].SourceNodeID = "node_missing"
		}},
		{"missing_field", func(value *cpctimelineModels.Transfer) {
			value.NodeMappings[0].Fields[0].TargetFieldID = "missing"
		}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			mapping := transferFixtureMapping()
			item.change(&mapping)
			if err := ValidateTransfer(
				transferFixtureNodes(),
				transferFixtureNodes(),
				mapping,
			); err == nil {
				t.Fatal("无效转移映射未被拒绝")
			}
		})
	}
}
