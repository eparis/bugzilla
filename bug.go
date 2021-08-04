package bugzilla

func (bug Bug) HasTargetReleae(targets []string) bool {
	for _, bugTarget := range bug.TargetRelease {
		for _, searchTarget := range targets {
			if bugTarget == searchTarget {
				return true
			}
		}
	}
	return false
}
