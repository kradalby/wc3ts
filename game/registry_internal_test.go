package game

import (
	"testing"
	"time"

	"github.com/nielsAD/gowarcraft3/protocol/w3gs"
)

func TestUpsert(t *testing.T) {
	t.Parallel()

	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := t0.Add(time.Minute)

	tests := []struct {
		name string
		old  Game
		obs  Game
		want Game
	}{
		{
			name: "first sighting",
			obs:  Game{Info: w3gs.GameInfo{SlotsUsed: 1}},
			want: Game{Info: w3gs.GameInfo{SlotsUsed: 1}, FirstSeen: t1, LastSeen: t1},
		},
		{
			name: "refresh keeps FirstSeen and takes new info",
			old:  Game{Info: w3gs.GameInfo{SlotsUsed: 1}, FirstSeen: t0, LastSeen: t0},
			obs:  Game{Info: w3gs.GameInfo{SlotsUsed: 2}},
			want: Game{Info: w3gs.GameInfo{SlotsUsed: 2}, FirstSeen: t0, LastSeen: t1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := upsert(tt.old, tt.obs, t1)
			if got.Info.SlotsUsed != tt.want.Info.SlotsUsed ||
				!got.FirstSeen.Equal(tt.want.FirstSeen) ||
				!got.LastSeen.Equal(tt.want.LastSeen) {
				t.Errorf("upsert() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
