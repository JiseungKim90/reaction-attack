// CKKS reaction attack in Lattigo.
package main

import (
	"flag"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/tuneinsight/lattigo/v6/circuits/ckks/dft"
	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/ring"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// parseSecret accepts p:<probability>, h:<weight>, or g[:<sigma>].
func parseSecret(spec string) (ring.DistributionParameters, string, bool, int) {
	switch {
	case strings.HasPrefix(spec, "p:"):
		p, err := strconv.ParseFloat(spec[2:], 64)
		if err != nil {
			panic(fmt.Errorf("bad -secret %q: %w", spec, err))
		}
		return ring.Ternary{P: p}, fmt.Sprintf("ternary-p%g", p), false, 1
	case strings.HasPrefix(spec, "h:"):
		h, err := strconv.Atoi(spec[2:])
		if err != nil {
			panic(fmt.Errorf("bad -secret %q: %w", spec, err))
		}
		return ring.Ternary{H: h}, fmt.Sprintf("ternary-h%d", h), false, 1
	case spec == "g" || strings.HasPrefix(spec, "g:"):
		sigma := 3.2
		if strings.HasPrefix(spec, "g:") {
			s, err := strconv.ParseFloat(spec[2:], 64)
			if err != nil {
				panic(fmt.Errorf("bad -secret %q: %w", spec, err))
			}
			sigma = s
		}
		bound := int(math.Ceil(6.0 * sigma))
		return ring.DiscreteGaussian{Sigma: sigma, Bound: float64(bound)}, fmt.Sprintf("gaussian-s%g", sigma), true, bound
	default:
		panic(fmt.Errorf("bad -secret %q: want p:<float>, h:<int>, or g[:<sigma>]", spec))
	}
}

func main() {
	logN := flag.Int("logn", 12, "log2 of ring dimension")
	logD := flag.Int("logd", 30, "log2 of CKKS scale Delta")
	logQ0 := flag.Int("logq0", 35, "log2 of the level-0 ciphertext modulus q_0 (matches paper Table 1 'log q')")
	measureC2S := flag.Bool("c2s-noise", false, "additionally measure the empirical B_C2S noise after CoeffsToSlots")
	c2sOnly := flag.Bool("c2s-only", false, "stop after the C2S measurement (requires -c2s-noise)")
	secretSpec := flag.String("secret", "p:0.666667", "secret distribution: p:<float> Bernoulli ternary, h:<int> fixed Hamming weight, g[:<sigma>] discrete Gaussian")
	noRlk := flag.Bool("no-rlk", false, "omit the relinearization key from the evaluator (validates the no-rlk claim: plaintext-ciphertext multiply must not need it)")
	flag.Parse()
	if *c2sOnly && !*measureC2S {
		panic("-c2s-only requires -c2s-noise")
	}

	xs, secretLabel, boundedSearch, secretBound := parseSecret(*secretSpec)

	fmt.Printf("=== Lattigo wire-format reaction attack (Algorithm 1) ===\n")
	fmt.Printf("logN=%d  logDelta=%d  secret=%s\n", *logN, *logD, secretLabel)

	// Chain: q0, four C2S primes, and one spare; upper primes track the scale.
	psize := 50
	if *logD > 50 {
		psize = *logD
	}
	c2sLevels := []int{psize, psize, psize, psize}
	spareLevels := []int{psize}
	logQ := append(append([]int{*logQ0}, c2sLevels...), spareLevels...)
	params, err := ckks.NewParametersFromLiteral(ckks.ParametersLiteral{
		LogN:            *logN,
		LogQ:            logQ,
		LogP:            []int{61, 61},
		LogDefaultScale: *logD,
		Xs:              xs,
	})
	if err != nil {
		panic(err)
	}
	N := 1 << params.LogN()
	ringQ := params.RingQ()
	delta := uint64(1) << *logD
	fmt.Printf("N=%d  logQ=%v  LogDefaultScale=%d  levels=%d  logSlots=%d\n",
		N, params.LogQ(), params.LogDefaultScale(), params.MaxLevel()+1, params.LogMaxSlots())

	kgen := rlwe.NewKeyGenerator(params)
	sk, pk := kgen.GenKeyPairNew()
	rlk := kgen.GenRelinearizationKeyNew(sk)
	dec := rlwe.NewDecryptor(params, sk)

	secret := extractSecret(sk, ringQ, N)
	hw := 0
	for _, v := range secret {
		if v != 0 {
			hw++
		}
	}
	fmt.Printf("Secret hamming weight = %d / %d\n", hw, N)

	// C2S keys.
	logSlots := params.LogMaxSlots()
	dftLevels := make([]int, len(c2sLevels))
	for i := range dftLevels {
		dftLevels[i] = 1
	}
	c2sLit := dft.MatrixLiteral{
		Type:     dft.HomomorphicEncode,
		Format:   dft.RepackImagAsReal,
		LogSlots: logSlots,
		LevelQ:   params.MaxLevelQ(),
		LevelP:   params.MaxLevelP(),
		Levels:   dftLevels,
	}
	encoder := ckks.NewEncoder(params)
	c2sMatrices, err := dft.NewMatrixFromLiteral(params, c2sLit, encoder)
	if err != nil {
		panic(fmt.Errorf("NewMatrixFromLiteral: %w", err))
	}
	galEls := append(c2sLit.GaloisElements(params),
		params.GaloisElementOrderTwoOrthogonalSubgroup())
	galKeys := kgen.GenGaloisKeysNew(galEls, sk)
	var relin *rlwe.RelinearizationKey
	if !*noRlk {
		relin = rlk
	}
	evk := rlwe.NewMemEvaluationKeySet(relin, galKeys...)
	fmt.Printf("relinearization key in evaluator: %v\n", relin != nil)
	eval := ckks.NewEvaluator(params, evk)
	dftEval := dft.NewEvaluator(params, eval)
	encryptor := rlwe.NewEncryptor(params, pk)
	_ = encryptor

	// ct1 = (a - Delta, b).
	ct1 := buildShiftedCt(params, pk, delta)

	// Check ct1 decrypts to Delta*s + e_pk.
	verifyShiftedCt(params, dec, ct1, secret, delta)

	// Coefficients to slots.
	t0 := time.Now()
	ct2Real, ct2Imag, err := dftEval.CoeffsToSlotsNew(ct1, c2sMatrices)
	if err != nil {
		panic(fmt.Errorf("CoeffsToSlots: %w", err))
	}
	fmt.Printf("C2S done in %.2fs  ctReal.Level=%d ctImag.Level=%d  scale=2^%.2f\n",
		time.Since(t0).Seconds(), ct2Real.Level(), levelOf(ct2Imag),
		math.Log2(ct2Real.Scale.Float64()))

	// Check the slot/coefficient map.
	verifyC2S(params, encoder, dec, ct2Real, ct2Imag, secret)

	// Measure C2S noise.
	if *measureC2S {
		measureC2SNoise(params, encoder, dec, ct2Real, ct2Imag, secret)
	}
	if *c2sOnly {
		return
	}

	// Recover every coefficient.
	recovered := make([]int, N)
	queries := 0
	tAttack := time.Now()
	for i := 0; i < N; i++ {
		realPart := i < N/2
		slotIdx := i
		ct2 := ct2Real
		if !realPart {
			slotIdx = i - N/2
			ct2 = ct2Imag
		}

		if boundedSearch {
			// Bounded-secret bisection.
			val, q, err := recoverCoeffBinary(params, encoder, encryptor, eval, ct2, slotIdx, secretBound, dec)
			if err != nil {
				panic(fmt.Errorf("binary-search i=%d: %w", i, err))
			}
			queries += q
			recovered[i] = val
			continue
		}

		alpha := chooseAlpha(N)

		valid, normLog, err := supportOracle(params, encoder, encryptor, eval, ct2, slotIdx, alpha, dec)
		if err != nil {
			panic(fmt.Errorf("support i=%d: %w", i, err))
		}
		queries++
		_ = normLog
		if valid {
			recovered[i] = 0
			continue
		}
		validSign, _, err := signOracle(params, encoder, encryptor, eval, ct2, slotIdx, alpha, dec)
		if err != nil {
			panic(fmt.Errorf("sign i=%d: %w", i, err))
		}
		queries++
		if validSign {
			recovered[i] = +1
		} else {
			recovered[i] = -1
		}
	}
	wall := time.Since(tAttack)

	// Undo CKKS bit-reversal to return coefficient order.
	logSlotsR := params.LogMaxSlots()
	recoveredCoeff := make([]int, N)
	for i := 0; i < N; i++ {
		realPart := i < N/2
		slotIdx := i
		if !realPart {
			slotIdx = i - N/2
		}
		coeff := bitReverse(slotIdx, logSlotsR)
		if !realPart {
			coeff += N / 2
		}
		recoveredCoeff[coeff] = recovered[i]
	}
	correct := 0
	for i := 0; i < N; i++ {
		if recoveredCoeff[i] == secret[i] {
			correct++
		}
	}
	slotCorrect := 0
	for i := 0; i < N; i++ {
		if recovered[i] == secret[i] {
			slotCorrect++
		}
	}
	fmt.Printf("\n=== Result ===\nRecovered (coeff): %d / %d  (%.2f%%)\nRecovered (slot, no perm): %d / %d  (%.2f%%)\nqueries=%d  ratio=%.3f  attack-wall=%s\n",
		correct, N, 100.0*float64(correct)/float64(N),
		slotCorrect, N, 100.0*float64(slotCorrect)/float64(N),
		queries, float64(queries)/float64(N), wall)
	// Machine-readable result.
	fmt.Printf("SUMMARY secret=%s logn=%d logq0=%d logd=%d N=%d hw=%d correct=%d queries=%d ratio=%.4f alpha=%d no_rlk=%t wall_seconds=%.4f\n",
		secretLabel, *logN, *logQ0, *logD, N, hw, correct, queries, float64(queries)/float64(N), chooseAlpha(N), *noRlk, wall.Seconds())
}

// measureC2SNoise reports slot residuals; scaled slot values are diagnostic only.
func measureC2SNoise(params ckks.Parameters, enc *ckks.Encoder, dec *rlwe.Decryptor, ctReal, ctImag *rlwe.Ciphertext, secret []int) {
	N := params.N()
	slots := params.MaxSlots()
	rr := make([]complex128, slots)
	ii := make([]complex128, slots)
	_ = enc.Decode(dec.DecryptNew(ctReal), rr)
	if ctImag != nil {
		_ = enc.Decode(dec.DecryptNew(ctImag), ii)
	}
	logS := params.LogMaxSlots()
	expectedReal := make([]complex128, slots)
	expectedImag := make([]complex128, slots)
	maxResid := 0.0
	sumSq := 0.0
	count := 0
	for k := 0; k < slots; k++ {
		coeffReal := bitReverse(k, logS)
		expectedReal[k] = complex(float64(secret[coeffReal]), 0)
		residReal := real(rr[k]) - float64(secret[coeffReal])
		ar := math.Abs(residReal)
		if ar > maxResid {
			maxResid = ar
		}
		sumSq += ar * ar
		count++
		if ctImag != nil {
			coeffImag := bitReverse(k, logS) + N/2
			expectedImag[k] = complex(float64(secret[coeffImag]), 0)
			residImag := real(ii[k]) - float64(secret[coeffImag])
			ai := math.Abs(residImag)
			if ai > maxResid {
				maxResid = ai
			}
			sumSq += ai * ai
			count++
		}
	}
	stddev := math.Sqrt(sumSq / float64(count))
	scale := ctReal.Scale.Float64()
	maxCoeff := maxCoefficientResidual(params, enc, dec, ctReal, expectedReal)
	if ctImag != nil {
		if candidate := maxCoefficientResidual(params, enc, dec, ctImag, expectedImag); candidate > maxCoeff {
			maxCoeff = candidate
		}
	}
	fmt.Printf("EMPIRICAL_BC2S max_slot_residual=%.3e std_slot_residual=%.3e scale=2^%.2f max_coeff_residual=%d log2_coeff_residual=%.2f\n",
		maxResid, stddev, math.Log2(scale), maxCoeff, math.Log2(float64(maxCoeff)))
}

// maxCoefficientResidual measures centered coefficient error modulo q0.
func maxCoefficientResidual(params ckks.Parameters, enc *ckks.Encoder, dec *rlwe.Decryptor, ct *rlwe.Ciphertext, expectedSlots []complex128) uint64 {
	actual := dec.DecryptNew(ct)
	expected := ckks.NewPlaintext(params, ct.Level())
	expected.MetaData = ct.MetaData.CopyNew()
	if err := enc.Encode(expectedSlots, expected); err != nil {
		panic(fmt.Errorf("encode expected C2S plaintext: %w", err))
	}
	ringQ := params.RingQ().AtLevel(ct.Level())
	diff := ringQ.NewPoly()
	ringQ.Sub(actual.Value, expected.Value, diff)
	if actual.IsNTT {
		ringQ.INTT(diff, diff)
	}
	if actual.IsMontgomery {
		ringQ.IMForm(diff, diff)
	}
	q0 := ringQ.SubRings[0].Modulus
	var max uint64
	for _, coefficient := range diff.Coeffs[0] {
		if coefficient > q0/2 {
			coefficient = q0 - coefficient
		}
		if coefficient > max {
			max = coefficient
		}
	}
	return max
}

func bitReverse(x, bits int) int {
	out := 0
	for i := 0; i < bits; i++ {
		out = (out << 1) | (x & 1)
		x >>= 1
	}
	return out
}

func chooseAlpha(N int) int64 {
	// tau=1; measured noise leaves margin at alpha=4.
	_ = N
	return 4
}

func levelOf(ct *rlwe.Ciphertext) int {
	if ct == nil {
		return -1
	}
	return ct.Level()
}

// extractSecret returns centered secret coefficients.
func extractSecret(sk *rlwe.SecretKey, ringQ *ring.Ring, N int) []int {
	cp := sk.Value.Q.CopyNew()
	ringQ.INTT(*cp, *cp)
	ringQ.IMForm(*cp, *cp)
	q0 := ringQ.SubRings[0].Modulus
	out := make([]int, N)
	for i := 0; i < N; i++ {
		x := cp.Coeffs[0][i]
		switch {
		case x == 0:
			out[i] = 0
		case x == 1:
			out[i] = 1
		case x == q0-1:
			out[i] = -1
		case x < q0/2:
			out[i] = int(x)
		default:
			out[i] = -int(q0 - x)
		}
	}
	return out
}

// buildShiftedCt returns ct1=(a-Delta,b) from pk=(-a*s+e,a).
func buildShiftedCt(params ckks.Parameters, pk *rlwe.PublicKey, delta uint64) *rlwe.Ciphertext {
	ringQ := params.RingQ()
	N := params.N()
	ct := rlwe.NewCiphertext(params, 1, params.MaxLevel())

	// c1 = -pk[1].
	for limb := 0; limb < params.MaxLevel()+1; limb++ {
		q := ringQ.SubRings[limb].Modulus
		copy(ct.Value[1].Coeffs[limb], pk.Value[1].Q.Coeffs[limb])
		for i := 0; i < N; i++ {
			if v := ct.Value[1].Coeffs[limb][i]; v != 0 {
				ct.Value[1].Coeffs[limb][i] = q - v
			}
		}
	}
	// Add Delta.
	deltaPoly := ringQ.NewPoly()
	for limb := 0; limb < params.MaxLevel()+1; limb++ {
		q := ringQ.SubRings[limb].Modulus
		dmod := delta % q
		for i := 0; i < N; i++ {
			deltaPoly.Coeffs[limb][i] = 0
		}
		deltaPoly.Coeffs[limb][0] = dmod
	}
	ringQ.NTT(deltaPoly, deltaPoly)
	ringQ.MForm(deltaPoly, deltaPoly)
	ringQ.Add(ct.Value[1], deltaPoly, ct.Value[1])

	// c0 = -pk[0].
	for limb := 0; limb < params.MaxLevel()+1; limb++ {
		q := ringQ.SubRings[limb].Modulus
		copy(ct.Value[0].Coeffs[limb], pk.Value[0].Q.Coeffs[limb])
		for i := 0; i < N; i++ {
			if v := ct.Value[0].Coeffs[limb][i]; v != 0 {
				ct.Value[0].Coeffs[limb][i] = q - v
			}
		}
	}
	// Store standard NTT form.
	ringQ.IMForm(ct.Value[0], ct.Value[0])
	ringQ.IMForm(ct.Value[1], ct.Value[1])
	ct.MetaData.IsNTT = true
	ct.MetaData.IsMontgomery = false
	ct.MetaData.IsBatched = true
	ct.MetaData.LogDimensions = ring.Dimensions{Rows: 0, Cols: params.LogMaxSlots()}
	ct.MetaData.Scale = rlwe.NewScale(math.Exp2(float64(params.LogDefaultScale())))
	return ct
}

func verifyShiftedCt(params ckks.Parameters, dec *rlwe.Decryptor, ct *rlwe.Ciphertext, secret []int, delta uint64) {
	ringQ := params.RingQ().AtLevel(ct.Level())
	pt := dec.DecryptNew(ct)
	cp := pt.Value.CopyNew()
	if pt.IsNTT {
		ringQ.INTT(*cp, *cp)
	}
	if pt.IsMontgomery {
		ringQ.IMForm(*cp, *cp)
	}
	q0 := ringQ.SubRings[0].Modulus
	match := 0
	maxDev := uint64(0)
	for i := 0; i < params.N(); i++ {
		x := cp.Coeffs[0][i]
		sv := int64(secret[i])
		mag := uint64(sv)
		if sv < 0 {
			mag = uint64(-sv)
		}
		prod := (mag % q0) * (delta % q0) % q0
		var expected uint64
		if sv >= 0 {
			expected = prod
		} else {
			expected = (q0 - prod) % q0
		}
		var diff uint64
		if x >= expected {
			diff = x - expected
		} else {
			diff = expected - x
		}
		if diff > q0/2 {
			diff = q0 - diff
		}
		if diff < (1 << 20) {
			match++
		}
		if diff > maxDev {
			maxDev = diff
		}
	}
	fmt.Printf("Step1 verify: coeffs within 2^20 of Delta*s: %d / %d   max dev = 2^%.2f\n",
		match, params.N(), math.Log2(float64(maxDev)+1.0))
}

func verifyC2S(params ckks.Parameters, enc *ckks.Encoder, dec *rlwe.Decryptor, ctReal, ctImag *rlwe.Ciphertext, secret []int) {
	slots := params.MaxSlots()
	rr := make([]float64, slots)
	ii := make([]float64, slots)
	_ = enc.Decode(dec.DecryptNew(ctReal), rr)
	if ctImag != nil {
		_ = enc.Decode(dec.DecryptNew(ctImag), ii)
	}
	N := params.N()
	logS := params.LogMaxSlots()
	for k := 0; k < 6; k++ {
		cr := bitReverse(k, logS)
		var imag float64
		if cr+N/2 < N {
			imag = ii[k]
		}
		fmt.Printf("  C2S slot %d:  real=%+.4f exp s_%d=%+d   imag=%+.4f exp s_%d=%+d", k, rr[k], cr, secret[cr], imag, cr+N/2, secret[cr+N/2])
		fmt.Println()
	}
	mismatch := 0
	tot := 0
	for k := 0; k < slots; k++ {
		cr := bitReverse(k, logS)
		if math.Abs(rr[k]-float64(secret[cr])) > 0.4 {
			mismatch++
		}
		tot++
		if ctImag != nil && cr+N/2 < N {
			if math.Abs(ii[k]-float64(secret[cr+N/2])) > 0.4 {
				mismatch++
			}
			tot++
		}
	}
	fmt.Printf("  C2S verify: %d / %d slot values within 0.4 of expected", tot-mismatch, tot)
	fmt.Println()
}

// supportOracle tests whether a slot is nonzero.
func supportOracle(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, ct2 *rlwe.Ciphertext, slotIdx int, alpha int64, dec *rlwe.Decryptor) (bool, float64, error) {
	mask := make([]complex128, params.MaxSlots())
	mask[slotIdx] = complex(float64(alpha), 0)
	ct3, err := eval.MulRelinNew(ct2, mask)
	if err != nil {
		return false, 0, err
	}
	ct0 := encryptor.EncryptZeroNew(ct3.Level())
	ct0.MetaData.IsBatched = ct3.MetaData.IsBatched
	ct0.MetaData.LogDimensions = ct3.MetaData.LogDimensions
	ct0.MetaData.Scale = ct3.MetaData.Scale
	if err := eval.Add(ct0, ct3, ct3); err != nil {
		return false, 0, err
	}
	return decryptionOracle(params, dec, ct3)
}

// signOracle tests a nonzero slot's sign.
func signOracle(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, ct2 *rlwe.Ciphertext, slotIdx int, alpha int64, dec *rlwe.Decryptor) (bool, float64, error) {
	mask := make([]complex128, params.MaxSlots())
	mask[slotIdx] = complex(float64(alpha), 0)
	pt := ckks.NewPlaintext(params, ct2.Level())
	pt.MetaData.LogDimensions = ring.Dimensions{Rows: 0, Cols: params.LogMaxSlots()}
	if err := encoder.Encode(mask, pt); err != nil {
		return false, 0, err
	}
	ct3, err := eval.MulRelinNew(ct2, pt)
	if err != nil {
		return false, 0, err
	}
	// Subtract Enc(alpha) at slot i.
	offMask := make([]complex128, params.MaxSlots())
	offMask[slotIdx] = complex(float64(alpha), 0)
	ptOff := ckks.NewPlaintext(params, ct3.Level())
	if err := encoder.Encode(offMask, ptOff); err != nil {
		return false, 0, err
	}
	ctOff, err := encryptor.EncryptNew(ptOff)
	if err != nil {
		return false, 0, err
	}
	if err := eval.Sub(ct3, ctOff, ct3); err != nil {
		return false, 0, err
	}
	ct0 := encryptor.EncryptZeroNew(ct3.Level())
	ct0.MetaData.IsBatched = ct3.MetaData.IsBatched
	ct0.MetaData.LogDimensions = ct3.MetaData.LogDimensions
	ct0.MetaData.Scale = ct3.MetaData.Scale
	if err := eval.Add(ct0, ct3, ct3); err != nil {
		return false, 0, err
	}
	return decryptionOracle(params, dec, ct3)
}

// intervalOracle tests |s-offset| <= 1/alpha using public material and one reaction bit.
func intervalOracle(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, ct2 *rlwe.Ciphertext, slotIdx int, alpha, offset float64, dec *rlwe.Decryptor) (bool, error) {
	mask := make([]complex128, params.MaxSlots())
	mask[slotIdx] = complex(alpha, 0)
	ct3, err := eval.MulRelinNew(ct2, mask)
	if err != nil {
		return false, err
	}
	if offset != 0 {
		offMask := make([]complex128, params.MaxSlots())
		offMask[slotIdx] = complex(alpha*offset, 0)
		ptOff := ckks.NewPlaintext(params, ct3.Level())
		ptOff.MetaData.LogDimensions = ring.Dimensions{Rows: 0, Cols: params.LogMaxSlots()}
		if err := encoder.Encode(offMask, ptOff); err != nil {
			return false, err
		}
		ctOff, err := encryptor.EncryptNew(ptOff)
		if err != nil {
			return false, err
		}
		if err := eval.Sub(ct3, ctOff, ct3); err != nil {
			return false, err
		}
	}
	ct0 := encryptor.EncryptZeroNew(ct3.Level())
	ct0.MetaData.IsBatched = ct3.MetaData.IsBatched
	ct0.MetaData.LogDimensions = ct3.MetaData.LogDimensions
	ct0.MetaData.Scale = ct3.MetaData.Scale
	if err := eval.Add(ct0, ct3, ct3); err != nil {
		return false, err
	}
	ok, _, err := decryptionOracle(params, dec, ct3)
	return ok, err
}

// recoverCoeffBinary bisects the integer range [-S,S].
func recoverCoeffBinary(params ckks.Parameters, encoder *ckks.Encoder, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, ct2 *rlwe.Ciphertext, slotIdx, S int, dec *rlwe.Decryptor) (int, int, error) {
	lo, hi := -S, S
	q := 0
	for lo < hi {
		mid := lo + (hi-lo)/2
		c := float64(lo+mid) / 2.0
		w := float64(mid-lo)/2.0 + 0.5
		alpha := 1.0 / w
		ok, err := intervalOracle(params, encoder, encryptor, eval, ct2, slotIdx, alpha, c, dec)
		if err != nil {
			return 0, q, err
		}
		q++
		if ok {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, q, nil
}

func decryptionOracle(params ckks.Parameters, dec *rlwe.Decryptor, ct *rlwe.Ciphertext) (bool, float64, error) {
	pt := ckks.NewPlaintext(params, ct.Level())
	pt.MetaData = ct.MetaData
	dec.Decrypt(ct, pt)
	encoder := ckks.NewEncoder(params)
	slots := make([]complex128, params.MaxSlots())
	if err := encoder.Decode(pt, slots); err != nil {
		return false, 0, err
	}
	maxMag := 0.0
	for _, v := range slots {
		m := math.Hypot(real(v), imag(v))
		if m > maxMag {
			maxMag = m
		}
	}
	threshold := 1.0
	return maxMag <= threshold, math.Log2(maxMag + 1.0), nil
}
