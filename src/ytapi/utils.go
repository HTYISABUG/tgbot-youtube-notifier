package ytapi

// IsLiveBroadcast ...
func IsLiveBroadcast(v *Video) bool {
	return v.LiveStreamingDetails != nil
}

// IsUpcomingLiveBroadcast ...
func IsUpcomingLiveBroadcast(v *Video) bool {
	return IsLiveBroadcast(v) &&
		v.LiveStreamingDetails.ActualEndTime == "" &&
		v.LiveStreamingDetails.ActualStartTime == "" &&
		v.LiveStreamingDetails.ScheduledStartTime != ""
}

// IsLiveLiveBroadcast ...
func IsLiveLiveBroadcast(v *Video) bool {
	return IsLiveBroadcast(v) &&
		v.LiveStreamingDetails.ActualEndTime == "" &&
		v.LiveStreamingDetails.ActualStartTime != ""
}

// IsCompletedLiveBroadcast ...
func IsCompletedLiveBroadcast(v *Video) bool {
	return IsLiveBroadcast(v) &&
		v.LiveStreamingDetails.ActualEndTime != ""
}
