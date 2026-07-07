//go:build !race

package workpool

func workPoolRaceDetectorEnabled() bool {
	return false
}
