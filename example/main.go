// +build !windows

package main

import (
	"crypto/rand"
	"time"

	"golang.org/x/crypto/nacl/box"

	_ "github.com/cmars/sigprof"
)

type message struct {
	nonce    *[24]byte
	contents []byte
}

func newNonce() *[24]byte {
	var result [24]byte
	_, err := rand.Reader.Read(result[:])
	if err != nil {
		panic(err)
	}
	return &result
}

func main() {
	messages := make(chan message)

	pubKey, privKey, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	// consumer
	go func() {
		for {
			select {
			case m := <-messages:
				_, ok := box.Open(nil, m.contents, m.nonce, pubKey, privKey)
				if !ok {
					panic("box.Open failed")
				}
			}
		}
	}()

	// producer
	for {
		plaintext := []byte(time.Now().String())
		nonce := newNonce()
		messages <- message{
			nonce:    nonce,
			contents: box.Seal(nil, plaintext, nonce, pubKey, privKey),
		}
	}
}
