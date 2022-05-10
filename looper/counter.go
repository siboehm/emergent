package looper

// Ctr combines an integer with a maximum value. It supports time tracking within looper.
type Ctr struct {
	Cur int `desc:"current counter value"`
	Max int `desc:"maximum counter value -- only used if > 0"`
}

// Incr increments the counter by 1. Does not interact with Max.
func (ct *Ctr) Incr() {
	ct.Cur++
}

// IsOverMax returns true if counter is at or over Max (only if Max > 0)
func (ct *Ctr) IsOverMax() bool {
	return ct.Max > 0 && ct.Cur >= ct.Max
}

// Set sets the Cur value if different from Cur, while preserving previous value.
// Returns true if changed
func (ct *Ctr) Set(cur int) bool {
	if ct.Cur == cur {
		return false
	}
	ct.Cur = cur
	return true
}
