package rain

//CheckError simple error-checking function
func CheckError(e error) {
	if e != nil {
		panic(e)
	}
}
