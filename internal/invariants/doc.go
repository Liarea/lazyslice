// Package invariants is the black-box suite. It runs the built lazyslice
// binary against a real source and a real target and asserts, from outside the
// process, the six properties ARCHITECTURE.md says a snapshot has:
//
//	I1  every foreign key in the target resolves
//	I2  no flagged column in the target carries a value the classifier flags
//	I3  same source, same secret, same config: a byte-identical target
//	I4  the source is unchanged
//	I5  the emitted lazyslice.yml, fed back in, reproduces the snapshot
//	I6  the root table holds exactly --take rows
//
// Nothing here imports a stage package, and nothing here calls core.Run. The
// only interface it uses is the one a user has: a binary, two connection URLs
// and the databases afterwards. That is deliberate — a suite that reached into
// the pipeline could be made to pass by the same mistake that made the
// pipeline wrong.
//
// Every test file is behind the `integration` build tag and needs a Docker
// endpoint (internal/testutil), so this file is the package's only untagged
// source and carries nothing but this comment.
package invariants
