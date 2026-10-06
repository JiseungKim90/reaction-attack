package main

import (
	"bufio"
	"fmt"
	"math"
	"os"

	"github.com/tuneinsight/lattigo/v6/core/rlwe"
	"github.com/tuneinsight/lattigo/v6/schemes/ckks"
)

// subsetOracle reports whether a masked subset contains a nonzero; it uses no rlk.
func subsetOracle(params ckks.Parameters, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, ct2 *rlwe.Ciphertext, slots []int, alpha int64, dec *rlwe.Decryptor) bool {
	mask := make([]complex128, params.MaxSlots())
	for _, k := range slots {
		mask[k] = complex(float64(alpha), 0)
	}
	ct3, err := eval.MulRelinNew(ct2, mask)
	if err != nil {
		panic(err)
	}
	ct0 := encryptor.EncryptZeroNew(ct3.Level())
	ct0.MetaData.IsBatched = ct3.MetaData.IsBatched
	ct0.MetaData.LogDimensions = ct3.MetaData.LogDimensions
	ct0.MetaData.Scale = ct3.MetaData.Scale
	if err := eval.Add(ct0, ct3, ct3); err != nil {
		panic(err)
	}
	valid, _, err := decryptionOracle(params, dec, ct3)
	if err != nil {
		panic(err)
	}
	return !valid
}

// groupTestSupport finds nonzero slots by adaptive binary splitting.
func groupTestSupport(params ckks.Parameters, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, ct2 *rlwe.Ciphertext, universe []int, alpha int64, dec *rlwe.Decryptor) ([]int, int) {
	queries := 0
	q := func(S []int) bool {
		queries++
		return subsetOracle(params, encryptor, eval, ct2, S, alpha, dec)
	}
	var defectives []int
	if len(universe) == 0 || !q(universe) {
		return defectives, queries
	}
	stack := [][]int{universe}
	for len(stack) > 0 {
		S := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(S) == 1 {
			defectives = append(defectives, S[0])
			continue
		}
		mid := len(S) / 2
		L, R := S[:mid], S[mid:]
		if q(L) {
			stack = append(stack, L)
			if q(R) {
				stack = append(stack, R)
			}
		} else {
			stack = append(stack, R)
		}
	}
	return defectives, queries
}

// exportLWE writes A_S*s_S=-b+e modulo q0; true s is included for verification.
func exportLWE(params ckks.Parameters, pk *rlwe.PublicKey, recovered map[int]bool, secret []int, path string) {
	ringQ := params.RingQ()
	N := params.N()
	q0 := ringQ.SubRings[0].Modulus
	aCp := pk.Value[1].Q.CopyNew()
	ringQ.INTT(*aCp, *aCp)
	ringQ.IMForm(*aCp, *aCp)
	bCp := pk.Value[0].Q.CopyNew()
	ringQ.INTT(*bCp, *bCp)
	ringQ.IMForm(*bCp, *bCp)
	a := aCp.Coeffs[0]
	b := bCp.Coeffs[0]
	var S []int
	for i := 0; i < N; i++ {
		if recovered[i] {
			S = append(S, i)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	defer w.Flush()
	fmt.Fprintf(w, "q0 %d\n", q0)
	fmt.Fprintf(w, "n %d\n", N)
	fmt.Fprintf(w, "h %d\n", len(S))
	fmt.Fprint(w, "support")
	for _, i := range S {
		fmt.Fprintf(w, " %d", i)
	}
	fmt.Fprint(w, "\na")
	for i := 0; i < N; i++ {
		fmt.Fprintf(w, " %d", a[i])
	}
	fmt.Fprint(w, "\nb")
	for i := 0; i < N; i++ {
		fmt.Fprintf(w, " %d", b[i])
	}
	fmt.Fprint(w, "\ns")
	for i := 0; i < N; i++ {
		fmt.Fprintf(w, " %d", secret[i])
	}
	fmt.Fprint(w, "\n")
	fmt.Printf("LWE_EXPORT path=%s q0=%d n=%d h=%d\n", path, q0, N, len(S))
}

// runGroupTest recovers support and exports the reduced LWE instance.
func runGroupTest(params ckks.Parameters, pk *rlwe.PublicKey, encryptor *rlwe.Encryptor, eval *ckks.Evaluator, dec *rlwe.Decryptor, ct2Real, ct2Imag *rlwe.Ciphertext, secret []int, alpha int64) {
	N := params.N()
	half := N / 2
	logS := params.LogMaxSlots()
	uni := func() []int {
		u := make([]int, half)
		for i := range u {
			u[i] = i
		}
		return u
	}
	defR, qR := groupTestSupport(params, encryptor, eval, ct2Real, uni(), alpha, dec)
	defI, qI := groupTestSupport(params, encryptor, eval, ct2Imag, uni(), alpha, dec)
	recovered := make(map[int]bool)
	for _, k := range defR {
		recovered[bitReverse(k, logS)] = true
	}
	for _, k := range defI {
		recovered[bitReverse(k, logS)+half] = true
	}
	trueSupport := make(map[int]bool)
	for i, v := range secret {
		if v != 0 {
			trueSupport[i] = true
		}
	}
	fp, fn := 0, 0
	for i := range recovered {
		if !trueSupport[i] {
			fp++
		}
	}
	for i := range trueSupport {
		if !recovered[i] {
			fn++
		}
	}
	h := len(trueSupport)
	totalQ := qR + qI
	ratio, target := 0.0, 0.0
	if h > 0 {
		ratio = float64(totalQ) / float64(h)
		target = float64(h) * math.Log2(float64(N)/float64(h))
	}
	fmt.Printf("GROUPTEST N=%d h=%d queries=%d (qR=%d qI=%d) queries_per_h=%.2f h_log2_N_over_h=%.0f exact_support=%v fp=%d fn=%d\n",
		N, h, totalQ, qR, qI, ratio, target, fp == 0 && fn == 0, fp, fn)
	if fp == 0 && fn == 0 {
		exportLWE(params, pk, recovered, secret,
			fmt.Sprintf("results_grouptest/lwe_N%d_h%d.txt", N, h))
	}
}
