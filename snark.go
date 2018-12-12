package snark

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"github.com/arnaucube/go-snark/bn128"
	"github.com/arnaucube/go-snark/fields"
	"github.com/arnaucube/go-snark/r1csqap"
)

type Setup struct {
	Toxic struct {
		T      *big.Int // trusted setup secret
		Ka     *big.Int // prover
		Kb     *big.Int // prover
		Kc     *big.Int // prover
		Kbeta  *big.Int
		Kgamma *big.Int
		RhoA   *big.Int
		RhoB   *big.Int
		RhoC   *big.Int
	}

	// public
	G1T [][3]*big.Int    // t encrypted in G1 curve
	G2T [][3][2]*big.Int // t encrypted in G2 curve
	Pk  struct {         // Proving Key pk:=(pkA, pkB, pkC, pkH)
		A  [][3]*big.Int
		B  [][3][2]*big.Int
		C  [][3]*big.Int
		Kp [][3]*big.Int
		Ap [][3]*big.Int
		Bp [][3]*big.Int
		Cp [][3]*big.Int
	}
	Vk struct {
		A     [3][2]*big.Int
		B     [3]*big.Int
		C     [3][2]*big.Int
		G1Kbg [3]*big.Int    // g1 * Kbeta * Kgamma
		G2Kbg [3][2]*big.Int // g2 * Kbeta * Kgamma
		G2Kg  [3][2]*big.Int // g2 * Kgamma
		Vkz   [3][2]*big.Int
	}
}

type Proof struct {
	PiA  [3]*big.Int
	PiAp [3]*big.Int
	PiB  [3][2]*big.Int
	PiBp [3]*big.Int
	PiC  [3]*big.Int
	PiCp [3]*big.Int
	PiH  [3]*big.Int
	PiKp [3]*big.Int
}

const bits = 512

func GenerateTrustedSetup(bn bn128.Bn128, pf r1csqap.PolynomialField, witnessLength int, alphas, betas, gammas [][]*big.Int, ax, bx, cx, hx, zx []*big.Int) (Setup, error) {
	var setup Setup
	var err error
	// generate random t value
	setup.Toxic.T, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}

	// k for calculating pi' and Vk
	setup.Toxic.Ka, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}
	setup.Toxic.Kb, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}
	setup.Toxic.Kc, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}

	// generate Kβ (Kbeta) and Kγ (Kgamma)
	setup.Toxic.Kbeta, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}
	setup.Toxic.Kgamma, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}

	// generate ρ (Rho): ρA, ρB, ρC
	setup.Toxic.RhoA, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}
	setup.Toxic.RhoB, err = rand.Prime(rand.Reader, bits)
	if err != nil {
		return Setup{}, err
	}
	setup.Toxic.RhoC = bn.Fq1.Mul(setup.Toxic.RhoA, setup.Toxic.RhoB)

	// encrypt t values with curve generators
	var gt1 [][3]*big.Int
	var gt2 [][3][2]*big.Int
	for i := 0; i < witnessLength; i++ {
		tPow := bn.Fq1.Exp(setup.Toxic.T, big.NewInt(int64(i)))
		tEncr1 := bn.G1.MulScalar(bn.G1.G, tPow)
		gt1 = append(gt1, tEncr1)
		tEncr2 := bn.G2.MulScalar(bn.G2.G, tPow)
		gt2 = append(gt2, tEncr2)
	}
	// gt1: g1, g1*t, g1*t^2, g1*t^3, ...
	// gt2: g2, g2*t, g2*t^2, ...
	setup.G1T = gt1
	setup.G2T = gt2

	setup.Vk.A = bn.G2.MulScalar(bn.G2.G, setup.Toxic.Ka)
	setup.Vk.B = bn.G1.MulScalar(bn.G1.G, setup.Toxic.Kb)
	setup.Vk.C = bn.G2.MulScalar(bn.G2.G, setup.Toxic.Kc)

	/*
		Verification keys:
		- Vk_betagamma1: setup.G1Kbg = g1 * Kbeta*Kgamma
		- Vk_betagamma2: setup.G2Kbg = g2 * Kbeta*Kgamma
		- Vk_gamma: setup.G2Kg = g2 * Kgamma
	*/
	kbg := bn.Fq1.Mul(setup.Toxic.Kbeta, setup.Toxic.Kgamma)
	setup.Vk.G1Kbg = bn.G1.MulScalar(bn.G1.G, kbg)
	setup.Vk.G2Kbg = bn.G2.MulScalar(bn.G2.G, kbg)
	setup.Vk.G2Kg = bn.G2.MulScalar(bn.G2.G, setup.Toxic.Kgamma)

	for i := 0; i < witnessLength; i++ {
		// A[i] = g1 * ax[t]
		at := pf.Eval(alphas[i], setup.Toxic.T)
		a := bn.G1.MulScalar(bn.G1.G, at)
		setup.Pk.A = append(setup.Pk.A, a)

		bt := pf.Eval(betas[i], setup.Toxic.T)
		bg1 := bn.G1.MulScalar(bn.G1.G, bt)
		bg2 := bn.G2.MulScalar(bn.G2.G, bt)
		setup.Pk.B = append(setup.Pk.B, bg2)

		ct := pf.Eval(gammas[i], setup.Toxic.T)
		c := bn.G1.MulScalar(bn.G1.G, ct)
		setup.Pk.C = append(setup.Pk.C, c)

		kt := bn.Fq1.Add(bn.Fq1.Add(at, bt), ct)
		k := bn.G1.MulScalar(bn.G1.G, kt)

		setup.Pk.Ap = append(setup.Pk.Ap, bn.G1.MulScalar(a, setup.Toxic.Ka))
		setup.Pk.Bp = append(setup.Pk.Bp, bn.G1.MulScalar(bg1, setup.Toxic.Kb))
		setup.Pk.Cp = append(setup.Pk.Cp, bn.G1.MulScalar(c, setup.Toxic.Kc))
		setup.Pk.Kp = append(setup.Pk.Kp, bn.G1.MulScalar(k, setup.Toxic.Kbeta))
	}
	setup.Vk.Vkz = bn.G2.MulScalar(bn.G2.G, pf.Eval(zx, setup.Toxic.T))

	return setup, nil
}

func GenerateProofs(bn bn128.Bn128, f fields.Fq, setup Setup, hx []*big.Int, w []*big.Int) (Proof, error) {
	var proof Proof
	proof.PiA = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	proof.PiAp = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	proof.PiB = bn.Fq6.Zero()
	proof.PiBp = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	proof.PiC = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	proof.PiCp = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	proof.PiH = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}
	proof.PiKp = [3]*big.Int{bn.G1.F.Zero(), bn.G1.F.Zero(), bn.G1.F.Zero()}

	for i := 0; i < len(w); i++ {
		proof.PiA = bn.G1.Add(proof.PiA, bn.G1.MulScalar(setup.Pk.A[i], w[i]))
		proof.PiAp = bn.G1.Add(proof.PiAp, bn.G1.MulScalar(setup.Pk.Ap[i], w[i]))
	}

	for i := 0; i < len(w); i++ {
		proof.PiB = bn.G2.Add(proof.PiB, bn.G2.MulScalar(setup.Pk.B[i], w[i]))
		proof.PiBp = bn.G1.Add(proof.PiBp, bn.G1.MulScalar(setup.Pk.Bp[i], w[i]))

		proof.PiC = bn.G1.Add(proof.PiC, bn.G1.MulScalar(setup.Pk.C[i], w[i]))
		proof.PiCp = bn.G1.Add(proof.PiCp, bn.G1.MulScalar(setup.Pk.Cp[i], w[i]))

		proof.PiKp = bn.G1.Add(proof.PiKp, bn.G1.MulScalar(setup.Pk.Kp[i], w[i]))
	}
	for i := 0; i < len(hx); i++ {
		proof.PiH = bn.G1.Add(proof.PiH, bn.G1.MulScalar(setup.G1T[i], hx[i]))
	}

	return proof, nil
}

func VerifyProof(bn bn128.Bn128, setup Setup, proof Proof) bool {

	// e(piA, Va) == e(piA', g2)
	pairingPiaVa, err := bn.Pairing(proof.PiA, setup.Vk.A)
	if err != nil {
		return false
	}
	pairingPiapG2, err := bn.Pairing(proof.PiAp, bn.G2.G)
	if err != nil {
		return false
	}
	if !bn.Fq12.Equal(pairingPiaVa, pairingPiapG2) {
		return false
	} else {
		fmt.Println("valid knowledge commitment for A")
	}

	// e(Vb, piB) == e(piB', g2)
	pairingVbPib, err := bn.Pairing(setup.Vk.B, proof.PiB)
	if err != nil {
		return false
	}
	pairingPibpG2, err := bn.Pairing(proof.PiBp, bn.G2.G)
	if err != nil {
		return false
	}
	if !bn.Fq12.Equal(pairingVbPib, pairingPibpG2) {
		return false
	} else {
		fmt.Println("valid knowledge commitment for B")
	}

	// e(piC, Vc) == e(piC', g2)
	pairingPicVc, err := bn.Pairing(proof.PiC, setup.Vk.C)
	if err != nil {
		return false
	}
	pairingPicpG2, err := bn.Pairing(proof.PiCp, bn.G2.G)
	if err != nil {
		return false
	}
	if !bn.Fq12.Equal(pairingPicVc, pairingPicpG2) {
		return false
	} else {
		fmt.Println("valid knowledge commitment for C")
	}

	// Vkx, to then calculate Vkx+piA

	// e(Vkx+piA, piB) == e(piH, Vkz) * e(piC, g2)
	pairingPiaPib, err := bn.Pairing(proof.PiA, proof.PiB)
	if err != nil {
		return false
	}
	pairingPihVkz, err := bn.Pairing(proof.PiH, setup.Vk.Vkz)
	if err != nil {
		return false
	}
	pairingPicG2, err := bn.Pairing(proof.PiC, bn.G2.G)
	if err != nil {
		return false
	}
	pairingR := bn.Fq12.Mul(pairingPihVkz, pairingPicG2)
	if !bn.Fq12.Equal(pairingPiaPib, pairingR) {
		fmt.Println("p4")
		return false
	} else {
		fmt.Println("QAP disibility checked")
	}

	// e(piA+piC, g2KbetaKgamma) * e(g1KbetaKgamma, piB)
	// == e(piK, g2Kgamma)
	// piApiC := bn.G1.Add(proof.PiA, proof.PiC)
	// pairingPiACG2Kbg, err := bn.Pairing(piApiC, setup.Vk.G2Kbg)
	// if err != nil {
	//         return false
	// }
	// pairingG1KbgPiB, err := bn.Pairing(setup.Vk.G1Kbg, proof.PiB)
	// if err != nil {
	//         return false
	// }
	// pairingL := bn.Fq12.Mul(pairingPiACG2Kbg, pairingG1KbgPiB)
	// pairingR, err := bn.Pairing(proof.PiKp, setup.Vk.G2Kg)
	// if err != nil {
	//         return false
	// }
	// if !bn.Fq12.Equal(pairingL, pairingR) {
	//         fmt.Println("p5")
	//         return false
	// }

	//

	// e(piA, piB) == e(piH, Vz) * e(piC, g2)
	// pairingPiaPib, err := bn.Pairing(proof.PiA, proof.PiB)
	// if err != nil {
	//         return false
	// }
	// pairingPihVz, err := bn.Pairing(proof.PiH, setup.Vk.Vkz)
	// if err != nil {
	//         return false
	// }
	// pairingPicG2, err := bn.Pairing(proof.PiC, bn.G2.G)
	// if err != nil {
	//         return false
	// }
	// if !bn.Fq12.Equal(pairingPiaPib, bn.Fq12.Mul(pairingPihVz, pairingPicG2)) {
	//         return false
	// }

	return true
}
