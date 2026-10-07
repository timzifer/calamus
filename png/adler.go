package png

const adlerBase = 65521

// adler32Combine returns the Adler-32 of A followed by B, given the
// Adler-32 of each and the length of B (zlib's adler32_combine).
func adler32Combine(a1, a2 uint32, len2 int) uint32 {
	rem := uint32(len2 % adlerBase)
	sum1 := a1 & 0xffff
	sum2 := (rem * sum1) % adlerBase
	sum1 += (a2 & 0xffff) + adlerBase - 1
	sum2 += (a1 >> 16) + (a2 >> 16) + adlerBase - rem
	if sum1 >= adlerBase {
		sum1 -= adlerBase
	}
	if sum1 >= adlerBase {
		sum1 -= adlerBase
	}
	if sum2 >= adlerBase<<1 {
		sum2 -= adlerBase << 1
	}
	if sum2 >= adlerBase {
		sum2 -= adlerBase
	}
	return sum1 | sum2<<16
}
