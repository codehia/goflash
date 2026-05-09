package scheduler

import "math"

type CardSchedule struct {
	EaseFactor   float64
	IntervalDays int
}

func GetCardSchedule(qualityScore, streakLength, intervalDays int, easeFactor float64) CardSchedule {
	var updatedCardSchedule CardSchedule

	if qualityScore >= 3 {
		switch streakLength {
		case 0:
			updatedCardSchedule.IntervalDays = 1
		case 1:
			updatedCardSchedule.IntervalDays = 6
		default:
			updatedCardSchedule.IntervalDays = int(math.Round(float64(intervalDays) * easeFactor))
		}
	} else {
		updatedCardSchedule.IntervalDays = 1
	}
	deficit := 5 - qualityScore
	adjustment := 0.1 - float64(deficit)*(0.08+float64(deficit)*0.02)
	newEaseFactor := float64(easeFactor) + adjustment
	if newEaseFactor < 1.3 {
		newEaseFactor = 1.3
	}
	updatedCardSchedule.EaseFactor = newEaseFactor
	return updatedCardSchedule
}
