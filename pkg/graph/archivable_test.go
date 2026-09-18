package graph

import (
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

func mkSt(id string, st item.Status, deps ...string) *item.Item {
	it := item.New(id, "t "+id, "b", deps, nil)
	it.SetStatus(st)
	return it
}

func ids(nodes []*Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Item.ID)
	}
	return out
}

func TestArchivable(t *testing.T) {
	cases := []struct {
		name  string
		items []*item.Item
		want  []string
	}{
		{"empty", nil, nil},
		{"all open", []*item.Item{mkSt("AWIT-TEST0001", item.StatusOpen)}, nil},
		{"lone closed", []*item.Item{mkSt("AWIT-TEST0001", item.StatusClosed)}, []string{"AWIT-TEST0001"}},
		{"closed chain fully archivable", []*item.Item{
			mkSt("AWIT-TEST0001", item.StatusClosed),
			mkSt("AWIT-TEST0002", item.StatusClosed, "AWIT-TEST0001"),
		}, []string{"AWIT-TEST0001", "AWIT-TEST0002"}},
		{"open dependant pins the whole chain", []*item.Item{
			mkSt("AWIT-TEST0001", item.StatusClosed),
			mkSt("AWIT-TEST0002", item.StatusClosed, "AWIT-TEST0001"),
			mkSt("AWIT-TEST0003", item.StatusOpen, "AWIT-TEST0002"),
			mkSt("AWIT-TEST0004", item.StatusClosed),
			mkSt("AWIT-TEST0005", item.StatusClosed, "AWIT-TEST0004"),
		}, []string{"AWIT-TEST0004", "AWIT-TEST0005"}},
		{"in_progress dependant pins", []*item.Item{
			mkSt("AWIT-TEST0001", item.StatusClosed),
			mkSt("AWIT-TEST0002", item.StatusInProgress, "AWIT-TEST0001"),
		}, nil},
		{"quarantined closed node is never archivable and pins its dep", []*item.Item{
			mkSt("AWIT-TEST0001", item.StatusClosed),
			mkSt("AWIT-TEST0002", item.StatusClosed, "AWIT-TEST0001", "AWIT-TEST9999"),
		}, nil},
		// A broken file has no deps edge, so it cannot pin anything;
		// Archivable ignores Broken entirely.
		{"broken file does not pin", []*item.Item{
			mkSt("AWIT-TEST0001", item.StatusClosed),
		}, []string{"AWIT-TEST0001"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var broken []item.Broken
			if tc.name == "broken file does not pin" {
				broken = []item.Broken{{ID: "AWIT-TEST0002", Reason: item.ReasonParse}}
			}
			g := Build(tc.items, broken)
			got := ids(g.Archivable())
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v, want %v", got, tc.want)
				}
			}
		})
	}
}
