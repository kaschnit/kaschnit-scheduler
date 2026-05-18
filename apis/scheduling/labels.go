package scheduling

const (
	// LabelKeyPrefix is the prefix of the labels for this plugin.
	LabelKeyPrefix = "quota." + GroupName + "/"
	// LabelKeyQueue is the name of the label whose value is the queue.
	LabelKeyQueue = LabelKeyPrefix + "queue"
	// LabelKeyPreemptor specifies whether this pod can preempt other pods.
	// If unspecified, empty, or invalid, defaults to false (this pod cannot preempt).
	LabelKeyPreemptor = LabelKeyPrefix + "preemptor"
	// LabelKeyVictim specifies whether this pod can be preempted by other pods.
	// If unspecified, empty, or invalid, defaults to false (this pod cannot be preempted).
	LabelKeyVictim = LabelKeyPrefix + "victim"
)
