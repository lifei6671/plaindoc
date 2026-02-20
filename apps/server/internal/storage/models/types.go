package models

type Role string

const (
	RoleOwner        Role = "owner"
	RoleCollaborator Role = "collaborator"
	RoleReader       Role = "reader"
)

type NodeType string

const (
	NodeTypeFolder NodeType = "folder"
	NodeTypeDoc    NodeType = "doc"
)

type RevisionSource string

const (
	RevisionSourceLocal  RevisionSource = "local"
	RevisionSourceRemote RevisionSource = "remote"
)

type Visibility string

const (
	VisibilityPublic        Visibility = "public"
	VisibilityAuthenticated Visibility = "authenticated"
	VisibilityMember        Visibility = "member"
)

func IsValidVisibility(value Visibility) bool {
	switch value {
	case VisibilityPublic, VisibilityAuthenticated, VisibilityMember:
		return true
	default:
		return false
	}
}

type AdminRole string

const (
	AdminRolePlatformAdmin AdminRole = "platform_admin"
	AdminRoleSpaceAdmin    AdminRole = "space_admin"
)

func IsValidAdminRole(value AdminRole) bool {
	switch value {
	case AdminRolePlatformAdmin, AdminRoleSpaceAdmin:
		return true
	default:
		return false
	}
}

type EntityStatus string

const (
	EntityStatusActive  EntityStatus = "active"
	EntityStatusBanned  EntityStatus = "banned"
	EntityStatusDeleted EntityStatus = "deleted"
)

func IsValidEntityStatus(value EntityStatus) bool {
	switch value {
	case EntityStatusActive, EntityStatusBanned, EntityStatusDeleted:
		return true
	default:
		return false
	}
}
