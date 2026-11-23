package httpapi

func battleEnemyScalingPlayerCount(players []battleUnitState, maxTeamSize int) int {
	count := 0
	for index := range players {
		if !battleUnitIsSummon(&players[index]) {
			count++
		}
	}
	if maxTeamSize > 0 {
		count = minInt(count, maxTeamSize)
	}
	return count
}
