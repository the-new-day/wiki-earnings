package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
)

func TestEditorEarnings_PayTo(t *testing.T) {
	tests := []struct {
		name     string
		earnings entity.EditorEarnings
		want     string
	}{
		{
			name:     "the payments nickname is paid when set",
			earnings: entity.EditorEarnings{Nickname: "tanker", PaymentsNickname: "Tanker_2007"},
			want:     "Tanker_2007",
		},
		{
			name:     "the wiki nickname is paid when none is set",
			earnings: entity.EditorEarnings{Nickname: "tanker"},
			want:     "tanker",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.earnings.PayTo())
		})
	}
}
