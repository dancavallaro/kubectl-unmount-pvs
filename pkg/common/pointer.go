package common

func StringP(val string) *string {
	return &val
}

func BoolP(val bool) *bool {
	return &val
}
