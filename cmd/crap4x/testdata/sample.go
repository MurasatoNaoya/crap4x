package sample

func simple(x int) int {
	return x + 1
}

func branchy(xs []int) int {
	total := 0
	for _, x := range xs {
		if x > 0 && x < 100 {
			total += x
		}
	}
	return total
}
