package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Parse()
	for _, arg := range flag.Args() {
		logfile := noerr(os.ReadFile(arg))
		key := bruteforceSnailKey(logfile)
		for i := range logfile {
			logfile[i] ^= key[i%len(key)]
		}
		fmt.Print(string(logfile))
	}

}

func bruteforceSnailKey(ciphertext []byte) []byte {
	// xortool -c 0x20 -l 128 log.clog
	const keyLength = 128
	key := make([]byte, keyLength)
	for keyPos := range keyLength {
		bestByte := byte(0)
		bestScore := 0
		for keyByte := range 256 {
			score := 0
			for cipherPos := keyPos; cipherPos < len(ciphertext); cipherPos += keyLength {
				if ciphertext[cipherPos]^byte(keyByte) == ' ' {
					score++
				}
			}
			if score > bestScore {
				bestScore = score
				bestByte = byte(keyByte)
			}
		}

		key[keyPos] = bestByte
	}
	return key
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func noerr[T any](ret T, err error) T {
	if err != nil {
		panic(err)
	}
	return ret
}
