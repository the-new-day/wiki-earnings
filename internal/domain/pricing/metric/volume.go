package metric

import (
	"math"

	"github.com/the-new-day/protanki-wiki-admin/internal/domain/entity"
)

const (
	// DefaultVolumeCap is the combined content size (prose words + weighted
	// table cells) that scores a full 1.0.
	DefaultVolumeCap = 2039

	// DefaultVolumeTableCellWeight scales sqrt(table cell count)
	// into word-equivalents.
	DefaultVolumeTableCellWeight = 16.70
)

// Volume scores how much content an article
// has - prose length plus table size - on a 0..1 scale.
type Volume struct {
	cap             float64
	tableCellWeight float64
}

func NewVolume(cap float64, tableCellWeight float64) *Volume {
	return &Volume{cap: cap, tableCellWeight: tableCellWeight}
}

// Units is the raw, uncapped content size: prose words plus table cells
// converted to word-equivalents.
func (v *Volume) Units(info *entity.ArticleInfo) float64 {
	return float64(len(info.Words)) + math.Sqrt(float64(TableCellCount(info.Wikitext)))*v.tableCellWeight
}

func (v *Volume) Score(info *entity.ArticleInfo) float64 {
	return Sat(v.Units(info), v.cap)
}
