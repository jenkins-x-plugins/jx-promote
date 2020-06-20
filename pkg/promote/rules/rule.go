package rules

// NewFunction creates a function based on the kind of rule
func NewFunction(r *PromoteRule) RuleFunction {
	spec := r.Config.Spec
	if spec.AppsRule != nil {
		return AppsRule
	}
	if spec.ChartRule != nil {
		return HelmRule
	}
	if spec.FileRule != nil {
		return FileRule
	}
	return nil
}
