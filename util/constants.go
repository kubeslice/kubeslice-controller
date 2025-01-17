package util

const (
	Tick    = string(rune(0x2705))
	Err     = string(rune(0x1F6AB))
	Sad     = string(rune(0x1F613))
	Wait    = string(rune(0x23F3))
	Watch   = string(rune(0x1F440))
	Find    = string(rune(0x1F50D))
	Bin     = string(rune(0x1F5D1))
	Recycle = string(rune(0x267D))
	Party   = string(rune(0x1F389))
)

const (
	ComponentController = "controller"
	InstanceController  = "controller"
	ClusterController   = "controller"
)

const (
	NotApplicable = "NA"
)

const (
	DefaultSliceQOSConfigName = "default"
	DefaultProjectSliceName   = "%s-default-slice"
)

const (
	LabelProjectName      = "kubeslice.io/project"
	LabelProjectNamespace = "kubeslice.io/project-namespace"
	LabelManagedBy        = "kubeslice.io/managed-by"
	LabelResourceOwner    = "kubeslice.io/resource-owner"
)

const ProjectKind = "Project"

const (
	LabelValueResourceOwner = "kubeslice-controller"
)
