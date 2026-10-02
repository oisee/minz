package mir2

func mustZ80Asm(asm string, err error) string {
	if err != nil {
		panic(err)
	}
	return asm
}
