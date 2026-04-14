package main

import (
	"fmt"
	"log"

	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multicodec"
	"github.com/multiformats/go-multihash"
)

func main() {
	// Datos de ejemplo
	data := []byte("hello world")

	// Crear CID v1 usando Prefix
	prefix := cid.Prefix{
		Version:  1,
		Codec:    uint64(multicodec.Raw),
		MhType:   multihash.SHA2_256,
		MhLength: -1,
	}

	cidV1, err := prefix.Sum(data)
	if err != nil {
		log.Fatalf("Error creando CID v1: %v", err)
	}
	fmt.Printf("CID v1 (Raw): %s\n", cidV1.String())

	// Crear CID v0 (solo SHA2-256, sin codec explícito)
	mh, err := multihash.Sum(data, multihash.SHA2_256, -1)
	if err != nil {
		log.Fatalf("Error creando multihash: %v", err)
	}
	cidV0 := cid.NewCidV0(mh)
	fmt.Printf("CID v0: %s\n", cidV0.String())

	// Verificar que son diferentes
	fmt.Printf("Son iguales: %t\n", cidV1.Equals(cidV0))

	// Parsear un CID de vuelta
	parsed, err := cid.Parse(cidV1.String())
	if err != nil {
		log.Fatalf("Error parseando CID: %v", err)
	}
	fmt.Printf("CID parseado: %s\n", parsed.String())
}
