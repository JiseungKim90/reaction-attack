// Lattigo implementation of Algorithm 1 from
// "Reaction Attack on CKKS and Its Variants".
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

// parseSecret maps a -secret spec to a Lattigo distribution.
//   p:<float>  Bernoulli ternary, Pr[s_i != 0] = float           (Lattigo default is p:0.333333)
//   h:<int>    fixed-Hamming-weight ternary, exactly <int> nonzero coefficients
//   g[:<sigma>] discrete Gaussian secret (default sigma=3.2), bound 6*sigma
// Returns the distribution, a self-describing label, whether to use the
// general bounded-secret binary-search recovery, and the bound S.
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
	secretSpec := flag.String("secret", "p:0.333333", "secret distribution: p:<float> Bernoulli ternary, h:<int> fixed Hamming weight, g[:<sigma>] discrete Gaussian")
	noRlk := flag.Bool("no-rlk", false, "omit the relinearization key from the evaluator (validates the no-rlk claim: plaintext-ciphertext multiply must not need it)")
	groupTest := flag.Bool("group-test", false, "group-testing support recovery; report query count vs h*log2(N/h)")
	flag.Parse()

	xs, secretLabel, boundedSearch, secretBound := parseSecret(*secretSpec)

	fmt.Printf("=== Lattigo wire-format reaction attack (Algorithm 1) ===\n")
	fmt.Printf("logN=%d  logDelta=%d  secret=%s\n", *logN, *logD, secretLabel)

	// LogQ = [q0, 50, 50, 50, 50, 50]: q0 from -logq0 flag, four 50-bit primes for C2S,
	// one 50-bit spare for the plaintext-ciphertext multiplication.
	// Prime sizes track the scale: 50-bit by default, but at least logDelta
	// so high-precision regimes (logDelta>50) rescale correctly. Lattigo
	// deployments do not always use a uniform 50-bit chain.
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

	// C2S matrices and Galois keys.
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

	// Step 1: build ct^(1) = (a - Delta, b).
	ct1 := buildShiftedCt(params, pk, delta)

	// Verify step 1: decryption gives Delta * s + e_pk.
	verifyShiftedCt(params, dec, ct1, secret, delta)

	// Step 2: CoeffsToSlots.
	t0 := time.Now()
	ct2Real, ct2Imag, err := dftEval.CoeffsToSlotsNew(ct1, c2sMatrices)
	if err != nil {
		panic(fmt.Errorf("CoeffsToSlots: %w", err))
	}
	fmt.Printf("C2S done in %.2fs  ctReal.Level=%d ctImag.Level=%d  scale=2^%.2f\n",
		time.Since(t0).Seconds(), ct2Real.Level(), levelOf(ct2Imag),
		math.Log2(ct2Real.Scale.Float64()))

	// Verify step 2: decoded slot k of ctReal ~ s_k; slot k of ctImag ~ s_{k+N/2}.
	verifyC2S(params, encoder, dec, ct2Real, ct2Imag, secret)

	// P4': empirical B_C2S measurement.
	if *measureC2S {
		measureC2SNoise(params, encoder, dec, ct2Real, ct2Imag, secret)
	}

	if *groupTest {
		runGroupTest(params, pk, encryptor, eval, dec, ct2Real, ct2Imag, secret, chooseAlpha(N))
		return
	}
	// Step 3+4: full attack loop.
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
			// General bounded-secret recovery: O(log S) interval-bisection
			// queries per coefficient (paper remark, Gaussian secrets).
			val, q, err := recoverCoeffBinary(params, encoder, encryptor, eval, ct2, slotIdx, secretBound, dec)
			if err != nil {
				panic(fmt.Errorf("binary-search i=%d: %w", i, err))
			}
			queries += q
			recovered[i] = val
			continue
		}

		alpha := chooseAlpha(N)

		// Support test.
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
		// Sign test.
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

	// Map attack outputs (slot domain) back to coefficient domain via
	// CKKS bit-reversal: slot k of the real/imag halves corresponds to
	// polynomial coefficient bitReverse(k, logSlots) and bitReverse(k, logSlots)+N/2.
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
	// Also count non-permutation-invariant slot-level accuracy.
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
	// Compact one-line summary for log scraping.
	fmt.Printf("SUMMARY secret=%s logn=%d logd=%d N=%d hw=%d correct=%d queries=%d ratio=%.4f wall_seconds=%.4f\n",
		secretLabel, *logN, *logD, N, hw, correct, queries, float64(queries)/float64(N), wall.Seconds())
}

// measureC2SNoise estimates B_C2S from (decoded slot - true secret) after
// rescaling by the post-C2S ciphertext scale.
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
	maxResid := 0.0
	sumSq := 0.0
	count := 0
	for k := 0; k < slots; k++ {
		coeffReal := bitReverse(k, logS)
		residReal := real(rr[k]) - float64(secret[coeffReal])
		ar := math.Abs(residReal)
		if ar > maxResid {
			maxResid = ar
		}
		sumSq += ar * ar
		count++
		if ctImag != nil {
			coeffImag := bitReverse(k, logS) + N/2
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
	fmt.Printf("EMPIRICAL_BC2S max_slot_residual=%.3e std_slot_residual=%.3e scale=2^%.2f  poly_max=2^%.2f  poly_std=2^%.2f  paper_bound=2^18\n",
		maxResid, stddev, math.Log2(scale), math.Log2(maxResid*scale+1.0), math.Log2(stddev*scale+1.0))
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
	// Constant slot-domain amplitude: any alpha* in (tau, tau/nu_alpha) works for the
	// slot-domain oracle (tau=1, nu_alpha<<1 measured); we fix alpha*=4. See paper Mask amplitude.
	_ = N
	return 4
	// Superseded coefficient-domain sizing alpha* = ceil(4 sqrt(N/2)) (inflates alpha*B_C2S at low precision):
	// a := int64(math.Ceil(4.0 * math.Sqrt(float64(N)/2.0)))
	// if a < 4 { a = 4 }
	// return a
}

func levelOf(ct *rlwe.Ciphertext) int {
	if ct == nil {
		return -1
	}
	return ct.Level()
}

// extractSecret recovers the ternary secret coefficients from sk.
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

// buildShiftedCt constructs ct^(1) = (a - Delta, b) given the public key
// pk = (-a*s + e, a) in Lattigo's NTT-Mont convention.
func buildShiftedCt(params ckks.Parameters, pk *rlwe.PublicKey, delta uint64) *rlwe.Ciphertext {
	ringQ := params.RingQ()
	N := params.N()
	ct := rlwe.NewCiphertext(params, 1, params.MaxLevel())

	// c1 := -pk[1] (in NTT-Mont)
	for limb := 0; limb < params.MaxLevel()+1; limb++ {
		q := ringQ.SubRings[limb].Modulus
		copy(ct.Value[1].Coeffs[limb], pk.Value[1].Q.Coeffs[limb])
		for i := 0; i < N; i++ {
			if v := ct.Value[1].Coeffs[limb][i]; v != 0 {
				ct.Value[1].Coeffs[limb][i] = q - v
			}
		}
	}
	// Add constant polynomial Delta (matched to NTT-Mont form of c1).
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

	// c0 := -pk[0] (in NTT-Mont)
	for limb := 0; limb < params.MaxLevel()+1; limb++ {
		q := ringQ.SubRings[limb].Modulus
		copy(ct.Value[0].Coeffs[limb], pk.Value[0].Q.Coeffs[limb])
		for i := 0; i < N; i++ {
			if v := ct.Value[0].Coeffs[limb][i]; v != 0 {
				ct.Value[0].Coeffs[limb][i] = q - v
			}
		}
	}
	// Convert NTT-Mont -> NTT-nonMont (standard ciphertext metadata).
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

// supportOracle implements the Step 4 support test.
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

// signOracle implements the sign test by subtracting an encryption of alpha at slot i.
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
	// Build encryption of "alpha at slot i" and subtract.
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

// intervalOracle isolates slot slotIdx of ct2 with scalar alpha, shifts by an
// encrypted offset alpha*offset, adds a fresh encryption of zero, and applies
// the same |slot| <= tau reaction oracle (tau = 1). The decoded slot carries
// alpha*(s - offset), so the oracle returns valid iff |s - offset| <= 1/alpha,
// i.e., iff the targeted coefficient lies in the symmetric integer interval of
// half-width 1/alpha centered at offset. Choosing alpha = 1/w realizes a
// membership test for any window, which is what the binary search bisects on.
// Only public material (pk via encryptor, Galois/relin keys via eval) and the
// oracle's accept/reject bit are used; the secret is never read here.
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

// recoverCoeffBinary recovers one integer secret coefficient s in [-S, S] by
// interval bisection over the candidate range, using only the accept/reject
// bit of intervalOracle. Each step tests membership in the left sub-interval
// [lo, mid] (center c = (lo+mid)/2, half-width w = (mid-lo)/2 + 1/2 so that the
// integers in [lo, mid] are inside and the next integer out is excluded), and
// uses alpha = 1/w. Returns the recovered value and the number of queries spent
// (ceil(log2(2S+1)) per coefficient, i.e., the paper's O(log S)).
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
	// Oracle decision: max |decoded slot| <= 1.
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
	return maxMag <= threshold, math.Log2(maxMag+1.0), nil
}
