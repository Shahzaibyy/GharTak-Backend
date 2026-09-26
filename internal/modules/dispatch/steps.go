package dispatch

func Steps(radiusKm int) []int {
	out := make([]int, 0, 3)
	for _, step := range []int{1, 3, 5} {
		if step <= radiusKm {
			out = append(out, step)
		}
	}
	return out
}
