package rain

//CheckError simple error-checking function
//probably shouldn't use this anymore
func CheckError(e error) {
	if e != nil {
		panic(e)
	}
}
