package urlapi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"

	"go.vocdoni.io/dvote/httprouter"
	"go.vocdoni.io/dvote/httprouter/bearerstdapi"
)

const (
	ChainHandler = "chain"
)

func (u *URLAPI) enableChainHandlers() error {
	if err := u.api.RegisterMethod(
		"/chain/organization/list",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.organizationListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/organization/list/{page}",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.organizationListHandler,
	); err != nil {
		return err
	}
	if err := u.api.RegisterMethod(
		"/chain/organization/count",
		"GET",
		bearerstdapi.MethodAccessTypePublic,
		u.organizationCountHandler,
	); err != nil {
		return err
	}

	return nil
}

// /chain/organization/list/<page>
// list the existing organizations
func (u *URLAPI) organizationListHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	var err error
	page := 0
	if ctx.URLParam("page") != "" {
		page, err = strconv.Atoi(ctx.URLParam("page"))
		if err != nil {
			return fmt.Errorf("cannot parse page number")
		}
	}

	page = page * MaxPageSize
	organization := &Organization{}
	list := u.scrutinizer.EntityList(MaxPageSize, page, "")
	for _, orgIDstr := range list {
		orgList := &OrganizationList{}
		orgList.OrganizationID, err = hex.DecodeString(orgIDstr)
		if err != nil {
			panic(err)
		}
		orgList.ElectionCount = u.scrutinizer.ProcessCount(orgList.OrganizationID)
		organization.Organizations = append(organization.Organizations, orgList)
	}

	var data []byte
	if data, err = json.Marshal(organization); err != nil {
		return err
	}

	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)
}

// /chain/organization/count
// return the number of organizations
func (u *URLAPI) organizationCountHandler(msg *bearerstdapi.BearerStandardAPIdata, ctx *httprouter.HTTPContext) error {
	count := u.scrutinizer.EntityCount()
	organization := &Organization{Count: &count}
	data, err := json.Marshal(organization)
	if err != nil {
		return err
	}
	return ctx.Send(data, bearerstdapi.HTTPstatusCodeOK)

}
