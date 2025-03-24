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
	LabelProjectNamespace = "kubeslice-project-namespace"
	// LabelProjectNamespace = "kubeslice.io/project-namespace" // TODO: implement this later
	LabelManagedBy = "kubeslice-controller-resource-name"
	// LabelManagedBy     = "kubeslice.io/managed-by" // TODO: implement this later
	LabelResourceOwner = "kubeslice-resource-owner"
	// LabelResourceOwner = "kubeslice.io/resource-owner" // TODO: implement this later
)

const ProjectKind = "Project"

const (
	LabelValueResourceOwner = "kubeslice-controller"
)
