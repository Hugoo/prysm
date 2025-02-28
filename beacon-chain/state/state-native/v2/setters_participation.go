package v2

import (
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stateutil"
)

// SetPreviousParticipationBits for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetPreviousParticipationBits(val []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[previousEpochParticipationBits].MinusRef()
	b.sharedFieldReferences[previousEpochParticipationBits] = stateutil.NewRef(1)

	b.previousEpochParticipation = val
	b.markFieldAsDirty(previousEpochParticipationBits)
	b.rebuildTrie[previousEpochParticipationBits] = true
	return nil
}

// SetCurrentParticipationBits for the beacon state. Updates the entire
// list to a new value by overwriting the previous one.
func (b *BeaconState) SetCurrentParticipationBits(val []byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.sharedFieldReferences[currentEpochParticipationBits].MinusRef()
	b.sharedFieldReferences[currentEpochParticipationBits] = stateutil.NewRef(1)

	b.currentEpochParticipation = val
	b.markFieldAsDirty(currentEpochParticipationBits)
	b.rebuildTrie[currentEpochParticipationBits] = true
	return nil
}

// AppendCurrentParticipationBits for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendCurrentParticipationBits(val byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	participation := b.currentEpochParticipation
	if b.sharedFieldReferences[currentEpochParticipationBits].Refs() > 1 {
		// Copy elements in underlying array by reference.
		participation = make([]byte, len(b.currentEpochParticipation))
		copy(participation, b.currentEpochParticipation)
		b.sharedFieldReferences[currentEpochParticipationBits].MinusRef()
		b.sharedFieldReferences[currentEpochParticipationBits] = stateutil.NewRef(1)
	}

	b.currentEpochParticipation = append(participation, val)
	b.markFieldAsDirty(currentEpochParticipationBits)
	b.addDirtyIndices(currentEpochParticipationBits, []uint64{uint64(len(b.currentEpochParticipation) - 1)})
	return nil
}

// AppendPreviousParticipationBits for the beacon state. Appends the new value
// to the the end of list.
func (b *BeaconState) AppendPreviousParticipationBits(val byte) error {
	b.lock.Lock()
	defer b.lock.Unlock()

	bits := b.previousEpochParticipation
	if b.sharedFieldReferences[previousEpochParticipationBits].Refs() > 1 {
		bits = make([]byte, len(b.previousEpochParticipation))
		copy(bits, b.previousEpochParticipation)
		b.sharedFieldReferences[previousEpochParticipationBits].MinusRef()
		b.sharedFieldReferences[previousEpochParticipationBits] = stateutil.NewRef(1)
	}

	b.previousEpochParticipation = append(bits, val)
	b.markFieldAsDirty(previousEpochParticipationBits)
	b.addDirtyIndices(previousEpochParticipationBits, []uint64{uint64(len(b.previousEpochParticipation) - 1)})

	return nil
}
