package run

import (
	"strings"

	"github.com/stroppy-io/stroppy-cloud/internal/domain/types"
)

// io-m3 disks must be sized in 93 GiB chunks (Yandex Cloud constraint).
// Keeping the rounding here means the SPA never has to think about it on
// the review step textarea.
const ydbStorageChunkGB = 93

// estimatedRawDataGB returns a coarse estimate of the user-data size in GB
// produced by `script × scaleFactor`, before headroom for WAL, compaction
// snapshots, etc. Heuristics:
//   - tpcc: ~100 MB per warehouse (data + indexes)
//   - tpcb: ~15 MB per scale unit
//   - anything else: treat like tpcc — closer to a generous default than 0
func estimatedRawDataGB(script string, scaleFactor int) int {
	if scaleFactor < 1 {
		scaleFactor = 1
	}
	mbPerUnit := 100
	if strings.HasPrefix(script, "tpcb") {
		mbPerUnit = 15
	}
	rawMB := mbPerUnit * scaleFactor
	rawGB := (rawMB + 1023) / 1024
	if rawGB < 1 {
		rawGB = 1
	}
	return rawGB
}

// CalculateYDBStoragePdiskGB returns the YDB storage pdisk size for the
// given script + scale factor: raw user data rounded up to the next 93 GiB
// chunk, then doubled for runtime headroom (WAL, compaction, snapshots).
//
// 500 TPC-C warehouses → 50 GB raw → 93 GB rounded → 186 GB doubled.
// 5000 TPC-C warehouses → 489 GB raw → 558 GB rounded → 1116 GB doubled.
func CalculateYDBStoragePdiskGB(script string, scaleFactor int) int {
	rawGB := estimatedRawDataGB(script, scaleFactor)
	chunks := (rawGB + ydbStorageChunkGB - 1) / ydbStorageChunkGB
	return chunks * ydbStorageChunkGB * 2
}

// AdjustYDBStorageDisk rewrites the YDB storage pdisk size based on the
// stroppy script + scale factor. Only the first secondary disk is touched
// (that's the YDB pdisk by convention; anything else added by the user is
// left alone).
//
// Called from the dry-run path so the size lands in the review-step
// textarea before the user gets a chance to edit it. Not called from
// Start — once the user submits, whatever size is on the wire wins.
func AdjustYDBStorageDisk(cfg *types.RunConfig) {
	if cfg.Database.Kind != types.DatabaseYDB || cfg.Database.YDB == nil {
		return
	}
	sd := cfg.Database.YDB.Storage.SecondaryDisks
	if len(sd) == 0 {
		return
	}
	cfg.Database.YDB.Storage.SecondaryDisks[0].SizeGB = CalculateYDBStoragePdiskGB(
		cfg.Stroppy.Script, cfg.Stroppy.ScaleFactor,
	)
}
