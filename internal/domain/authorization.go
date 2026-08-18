package domain

type Action string

const (
	ActionCatalogWrite    Action = "catalog_write"
	ActionPlanShipment    Action = "plan_shipment"
	ActionMoveShipment    Action = "move_shipment"
	ActionResolveHandoff  Action = "resolve_handoff"
	ActionRecordTelemetry Action = "record_telemetry"
	ActionReviewExcursion Action = "review_excursion"
	ActionReadOperations  Action = "read_operations"
	ActionReadAudit       Action = "read_audit"
)

var roleActions = map[Role]map[Action]bool{
	RoleOperations: {ActionCatalogWrite: true, ActionPlanShipment: true, ActionMoveShipment: true, ActionResolveHandoff: true, ActionRecordTelemetry: true, ActionReadOperations: true, ActionReadAudit: true},
	RoleCourier:    {ActionMoveShipment: true, ActionResolveHandoff: true, ActionRecordTelemetry: true, ActionReadOperations: true},
	RoleReviewer:   {ActionReviewExcursion: true, ActionReadOperations: true},
	RoleAuditor:    {ActionReadOperations: true, ActionReadAudit: true},
}

func (p Principal) CanAction(action Action) bool {
	return roleActions[p.Role][action]
}
