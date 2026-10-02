package mir2_test

func mustZ80Asm(asm string, err error) string {
	if err != nil {
		panic(err)
	}
	return asm
}
