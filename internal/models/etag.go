package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// BirdsETag computes a strong, order-sensitive ETag over the bird reference list.
func BirdsETag(birds []Bird) string {
	h := sha256.New()
	for _, b := range birds {
		h.Write([]byte(b.ID))
		h.Write([]byte{0})
		h.Write([]byte(b.CommonName))
		h.Write([]byte{0})
		h.Write([]byte(b.ScientificName))
		h.Write([]byte{0})
		if b.EbirdCode != nil {
			h.Write([]byte(*b.EbirdCode))
		}
		h.Write([]byte{0})
		if b.TaxonomicOrder != nil {
			h.Write([]byte(strconv.Itoa(int(*b.TaxonomicOrder))))
		}
		h.Write([]byte{0})
	}
	// Strong validator, quoted per RFC 7232.
	return `"` + hex.EncodeToString(h.Sum(nil)) + `"`
}
