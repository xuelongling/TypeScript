// SPDX-License-Identifier: Apache-2.0
// Candidate-local proof writes commit together with their selected relation.
package main

type arrayGenericProofChange struct {
	pair    [2]string
	prior   arrayGenericCompatibilityWitness
	existed bool
}

func (r *arrayGenericViews) beginProofCandidate() int {
	r.proofScopes++
	return len(r.proofUndo)
}

func (r *arrayGenericViews) storeProof(pair [2]string, proof arrayGenericCompatibilityWitness) error {
	// Reserve the undo work before mutation. Failed/canceled candidates must
	// still restore every write, without cancellation interrupting the rollback.
	if err := r.p.annotationWork(); err != nil {
		return err
	}
	if r.proofScopes > 0 {
		prior, existed := r.proofs[pair]
		r.proofUndo = append(r.proofUndo, arrayGenericProofChange{pair: pair, prior: prior, existed: existed})
	}
	r.proofs[pair] = proof
	return nil
}

func (r *arrayGenericViews) finishProofCandidate(mark int, selected bool) {
	if !selected {
		for i := len(r.proofUndo) - 1; i >= mark; i-- {
			change := r.proofUndo[i]
			if change.existed {
				r.proofs[change.pair] = change.prior
			} else {
				delete(r.proofs, change.pair)
			}
		}
		r.proofUndo = r.proofUndo[:mark]
	}
	r.proofScopes--
	// A successful nested candidate remains in its parent's undo scope.
	if r.proofScopes == 0 {
		r.proofUndo = nil
	}
}
