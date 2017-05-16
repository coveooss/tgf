package main

// Must traps errors and return the remaining results to the caller
// If there is an error, a panic is issued
func Must(result ...interface{}) interface{} {
	last := len(result) - 1
	err := result[last]
	if err != nil {
		panic(err)
	}

	result = result[:last]
	switch len(result) {
	case 0:
		return nil
	case 1:
		return result[0]
	default:
		return result
	}
}

func PanicOnError(err error) {
	if err != nil {
		panic(err)
	}
}
