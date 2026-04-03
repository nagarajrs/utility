// Package transfer – delta.go
//
// PLACEHOLDER: Block-level delta sync (planned for a future phase).
//
// Algorithm outline (not yet implemented):
//
//  1. The destination is divided into fixed-size blocks (e.g. 512 bytes).
//  2. For each block, two checksums are computed:
//     - A rolling weak checksum (Adler-32, same family as rsync) so the
//       receiver can slide a window across the file in O(n) time.
//     - A strong checksum (MD4, as used in rsync) to confirm a match.
//  3. The source sends only the blocks whose weak+strong checksums do NOT
//     match any block in the destination — i.e. only the changed regions.
//  4. The destination reassembles the file from matching blocks + new data.
//
// References:
//   - Andrew Tridgell's rsync algorithm paper (1996):
//     https://www.samba.org/~tridge/phd_thesis.pdf
//   - golang.org/x/crypto/md4  (for the strong checksum)

package transfer
