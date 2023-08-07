// Adapted from: https://gist.github.com/miguelmota/3ea9286bd1d3c2a985b67cac4ba2130a

package rsacrypto

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"fmt"
)

const KeySize = 2048
const Limiter = "\n\t"

// Generates private and public key
func GenerateKeyPair(bits int) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privkey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}
	return privkey, &privkey.PublicKey, nil
}

// Convert private key to bytes (pem)
func PrivateKeyToBytes(priv *rsa.PrivateKey) []byte {
	return x509.MarshalPKCS1PrivateKey(priv)
}

// Convert public key to bytes (pem)
func PublicKeyToBytes(pub *rsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(pub)
}

// Convert bytes (pem) to private key
func BytesToPrivateKey(priv []byte) (*rsa.PrivateKey, error) {
	return x509.ParsePKCS1PrivateKey(priv)
}

// Convert bytes (pem) to public key
func BytesToPublicKey(pub []byte) (*rsa.PublicKey, error) {
	pb, err := x509.ParsePKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}

	if pub, ok := pb.(*rsa.PublicKey); ok {
		return pub, nil
	}

	return nil, fmt.Errorf("Expected *rsa.PublicKey, got %T", pb)
}

func chunkBy[T any](items []T, chunkSize int) (chunks [][]T) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

// Encrypt message using public key
func EncryptWithPublicKey(msg []byte, pub *rsa.PublicKey) ([]byte, error) {
	hash := sha512.New()

	// Chunk the message into smaller parts
	var chunkSize = pub.N.BitLen()/8 - 2*hash.Size() - 2
	var encryptedChunks [][]byte
	chunks := chunkBy[byte](msg, chunkSize)
	for _, chunk := range chunks {
		ciphertext, err := rsa.EncryptOAEP(hash, rand.Reader, pub, chunk, nil)
		if err != nil {
			return []byte{}, err
		}
		encryptedChunks = append(encryptedChunks, ciphertext)
	}

	return bytes.Join(encryptedChunks, []byte(Limiter)), nil
}

// Decrypt message using private key
func DecryptWithPrivateKey(ciphertext []byte, priv *rsa.PrivateKey) ([]byte, error) {
	hash := sha512.New()
	dec_msg := []byte("")

	for _, chnk := range bytes.Split(ciphertext, []byte(Limiter)) {
		plaintext, err := rsa.DecryptOAEP(hash, rand.Reader, priv, chnk, nil)
		if err != nil {
			return []byte{}, err
		}
		dec_msg = append(dec_msg, plaintext...)
	}

	return dec_msg, nil
}

// Example
func main() {
	msg := "Hello, world!"
	PrKey, PubKey, _ := GenerateKeyPair(2048)
	prkstr := PrivateKeyToBytes(PrKey)
	fmt.Println("PrivateKey: ", string(prkstr))
	pbkstr, _ := PublicKeyToBytes(PubKey)
	fmt.Println("PublicKey: ", string(pbkstr))
	dePub, _ := BytesToPublicKey(pbkstr)
	enc, _ := EncryptWithPublicKey([]byte(msg), dePub)
	fmt.Println("Encrypted: ", string(enc))
	dePrv, _ := BytesToPrivateKey(prkstr)
	dec, _ := DecryptWithPrivateKey(enc, dePrv)
	fmt.Println("Decrypted: ", string(dec))
}
