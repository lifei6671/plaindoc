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
