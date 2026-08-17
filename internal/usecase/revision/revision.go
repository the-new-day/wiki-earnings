package revision

import "time"

type UseCase struct {
	now func() time.Time
}
