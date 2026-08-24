package discord

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/the-new-day/wiki-earnings/internal/domain/entity"
	"github.com/the-new-day/wiki-earnings/internal/usecase/earnings"
)

var payrollReport = earnings.Report{
	Editors: []entity.EditorEarnings{
		{EditorID: 1, Nickname: "tanker", PaymentsNickname: "Tanker_2007", Total: 60000},
		{EditorID: 2, Nickname: "hornet", Total: 40000},
	},
}

func TestNicknamesInPayrollOutput(t *testing.T) {
	period := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		format  func(earnings.Report) string
		want    []string
		notWant []string
	}{
		{
			name:    "the commands pay the game account",
			format:  formatCommands,
			want:    []string{"/givecry Tanker_2007 60000", "/addpremium Tanker_2007 3"},
			notWant: []string{"/givecry tanker ", "/addpremium tanker "},
		},
		{
			name:   "the commands fall back to the wiki nickname",
			format: formatCommands,
			want:   []string{"/givecry hornet 40000"},
		},
		{
			// The payments nickname is for paying, not for identifying: the
			// report is read against the wiki.
			name:    "the report keeps the wiki nickname",
			format:  func(report earnings.Report) string { return formatReport(period, report) },
			want:    []string{"tanker", "hornet"},
			notWant: []string{"Tanker_2007"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.format(payrollReport)

			for _, want := range tt.want {
				assert.Contains(t, got, want)
			}
			for _, notWant := range tt.notWant {
				assert.NotContains(t, got, notWant)
			}
		})
	}
}
