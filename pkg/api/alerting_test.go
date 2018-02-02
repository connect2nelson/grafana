package api

import (
	"testing"

	"github.com/grafana/grafana/pkg/api/dtos"
	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/middleware"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/util"

	. "github.com/smartystreets/goconvey/convey"
)

func TestAlertingApiEndpoint(t *testing.T) {
	Convey("Given an alert in a dashboard with an acl", t, func() {
		setupSingleAlertHandlers()

		viewerRole := m.ROLE_VIEWER
		editorRole := m.ROLE_EDITOR

		aclMockResp := []*m.DashboardAclInfoDTO{}
		bus.AddHandler("test", func(query *m.GetDashboardAclInfoListQuery) error {
			query.Result = aclMockResp
			return nil
		})

		bus.AddHandler("test", func(query *m.GetTeamsByUserQuery) error {
			query.Result = []*m.Team{}
			return nil
		})

		Convey("When user is editor and not in the ACL", func() {
			Convey("Should not be able to pause the alert", func() {
				cmd := dtos.PauseAlertCommand{
					AlertId: 1,
					Paused:  true,
				}
				postAlertScenario("When calling POST on", "/api/alerts/1/pause", "/api/alerts/:alertId/pause", m.ROLE_EDITOR, cmd, func(sc *scenarioContext) {
					CallPauseAlert(sc)
					So(sc.resp.Code, ShouldEqual, 403)
				})
			})
			Convey("Should not be able to fetch Alert States", func() {
				loggedInUserScenarioWithRole("When calling GET on", "GET", "/api/alerts/states-for-dashboard", "/api/alerts/states-for-dashboard", m.ROLE_EDITOR, func(sc *scenarioContext) {
					sc.handlerFunc = GetAlertStatesForDashboard
					sc.fakeReqWithParams("GET", sc.url, map[string]string{"dashboardId": "1"}).exec()

					So(sc.resp.Code, ShouldEqual, 403)
				})
			})
		})

		Convey("When user is editor and dashboard has default ACL", func() {
			aclMockResp = []*m.DashboardAclInfoDTO{
				{Role: &viewerRole, Permission: m.PERMISSION_VIEW},
				{Role: &editorRole, Permission: m.PERMISSION_EDIT},
			}

			Convey("Should be able to pause the alert", func() {
				cmd := dtos.PauseAlertCommand{
					AlertId: 1,
					Paused:  true,
				}
				postAlertScenario("When calling POST on", "/api/alerts/1/pause", "/api/alerts/:alertId/pause", m.ROLE_EDITOR, cmd, func(sc *scenarioContext) {
					CallPauseAlert(sc)
					So(sc.resp.Code, ShouldEqual, 200)
				})
			})

			Convey("Should be able to fetch Alert States", func() {
				loggedInUserScenarioWithRole("When calling GET on", "GET", "/api/alerts/states-for-dashboard", "/api/alerts/states-for-dashboard", m.ROLE_EDITOR, func(sc *scenarioContext) {
					sc.handlerFunc = GetAlertStatesForDashboard
					sc.fakeReqWithParams("GET", sc.url, map[string]string{"dashboardId": "1"}).exec()

					So(sc.resp.Code, ShouldEqual, 200)

					respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
					So(err, ShouldBeNil)
					So(len(respJSON.MustArray()), ShouldEqual, 2)
					So(respJSON.GetIndex(0).Get("state").MustString(), ShouldEqual, string(m.AlertStateOK))
					So(respJSON.GetIndex(1).Get("state").MustString(), ShouldEqual, string(m.AlertStateAlerting))
				})
			})
		})

	})

	Convey("Given a list of alerts", t, func() {
		alerts := []*m.Alert{
			{Id: 1, DashboardId: 1, OrgId: 1},
			{Id: 2, DashboardId: 2, OrgId: 1},
		}
		bus.AddHandler("test", func(query *m.GetAlertsQuery) error {
			query.Result = alerts
			return nil
		})

		dashboards := []*m.Dashboard{
			{Id: 1, Slug: "Dash1", Uid: util.GenerateShortUid()},
			{Id: 2, Slug: "Dash2", Uid: util.GenerateShortUid()},
		}
		bus.AddHandler("test", func(query *m.GetDashboardsQuery) error {
			query.Result = dashboards
			return nil
		})

		mockPermissions := []*m.DashboardPermissionForUser{}
		bus.AddHandler("test", func(query *m.GetDashboardPermissionsForUserQuery) error {
			query.Result = mockPermissions
			return nil
		})

		Convey("When user is editor and not in the ACL", func() {
			Convey("Should not be able see any alerts", func() {
				loggedInUserScenarioWithRole("When calling GET on", "GET", "/api/alerts", "/api/alerts", m.ROLE_EDITOR, func(sc *scenarioContext) {
					sc.handlerFunc = GetAlerts
					sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()

					So(sc.resp.Code, ShouldEqual, 200)

					respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
					So(err, ShouldBeNil)
					So(len(respJSON.MustArray()), ShouldEqual, 0)
				})
			})
		})

		Convey("When user is editor and is in the ACL for the two dashboards", func() {
			mockPermissions = append(mockPermissions, &m.DashboardPermissionForUser{
				DashboardId: 1, Permission: 1, PermissionName: "View",
			})
			mockPermissions = append(mockPermissions, &m.DashboardPermissionForUser{
				DashboardId: 2, Permission: 2, PermissionName: "Edit",
			})

			Convey("Should be able see alerts with permission level", func() {
				loggedInUserScenarioWithRole("When calling GET on", "GET", "/api/alerts", "/api/alerts", m.ROLE_EDITOR, func(sc *scenarioContext) {
					sc.handlerFunc = GetAlerts
					sc.fakeReqWithParams("GET", sc.url, map[string]string{}).exec()

					So(sc.resp.Code, ShouldEqual, 200)

					respJSON, err := simplejson.NewJson(sc.resp.Body.Bytes())
					So(err, ShouldBeNil)
					So(len(respJSON.MustArray()), ShouldEqual, 2)
					So(respJSON.GetIndex(0).Get("canEdit").MustBool(), ShouldBeFalse)
					So(respJSON.GetIndex(1).Get("canEdit").MustBool(), ShouldBeTrue)
				})
			})
		})
	})
}

func setupSingleAlertHandlers() {
	bus.AddHandler("test", func(query *m.GetAlertByIdQuery) error {
		query.Result = &m.Alert{Id: 1, DashboardId: 1, Name: "singlealert"}
		return nil
	})

	bus.AddHandler("test", func(query *m.GetAlertStatesForDashboardQuery) error {
		query.Result = []*m.AlertStateInfoDTO{
			{Id: 1, DashboardId: 1, PanelId: 1, State: m.AlertStateOK},
			{Id: 1, DashboardId: 1, PanelId: 2, State: m.AlertStateAlerting},
		}
		return nil
	})
}

func CallPauseAlert(sc *scenarioContext) {
	bus.AddHandler("test", func(cmd *m.PauseAlertCommand) error {
		return nil
	})

	sc.fakeReqWithParams("POST", sc.url, map[string]string{}).exec()
}

func postAlertScenario(desc string, url string, routePattern string, role m.RoleType, cmd dtos.PauseAlertCommand, fn scenarioFunc) {
	Convey(desc+" "+url, func() {
		defer bus.ClearBusHandlers()

		sc := setupScenarioContext(url)
		sc.defaultHandler = wrap(func(c *middleware.Context) Response {
			sc.context = c
			sc.context.UserId = TestUserID
			sc.context.OrgId = TestOrgID
			sc.context.OrgRole = role

			return PauseAlert(c, cmd)
		})

		sc.m.Post(routePattern, sc.defaultHandler)

		fn(sc)
	})
}
