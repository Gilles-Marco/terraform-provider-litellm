package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gzamboni/terraform-provider-litellm/provider/litellm"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var SchemaTeamMembership = map[string]*schema.Schema{
	"team_id": {
		Type:        schema.TypeString,
		Required:    true,
		Description: "Team ID",
	},
	"user_id": {
		Type:        schema.TypeString,
		Required:    true,
		Description: "User ID",
	},
	"role": {
		Type:        schema.TypeString,
		Required:    true,
		Description: "Litellm Role",
		ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
			value := val.(string)
			role, isValidated := litellm.ValidateRole(value)
			if !isValidated {
				errs = append(errs, fmt.Errorf("Provided role should be in this list %v", litellm.ROLE_LIST))
			}
			if role == litellm.PROXY_ADMIN || role == litellm.PROXY_ADMIN_VIEWER {
				errs = append(errs, fmt.Errorf("proxy_admin and proxy_admin_viewer cannot be set to associate a team with an user"))
			}

			return warns, errs
		},
	},
	// Max budget in theam doesnt anything for now (2024-11-15) so its better not to be set. When getting a team user are returned through the member_with_roles attribute, that doesn't include any mention of a budget so it doesnt work
	// Users with a budget should be returned through the team_memberships but that's not the case
	// "max_budget_in_team": {
	// 	Type:        schema.TypeFloat,
	// 	Required:    false,
	// 	Optional:    true,
	// 	Description: "Allowed budget in the team",
	// },
}

func resourceTeamMembership() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceTeamMembershipCreate,
		ReadContext:   resourceTeamMembershipRead,
		UpdateContext: resourceTeamMembershipUpdate,
		DeleteContext: resourceTeamMembershipDelete,
		Schema:        SchemaTeamMembership,
	}
}

type TeamMembershipData struct {
	UserId          string
	Role            string
	TeamId          string
	MaxBudgetInTeam float64
}

func getTeamMembershipData(d *schema.ResourceData) TeamMembershipData {
	userId := d.Get("user_id").(string)
	role := d.Get("role").(string)
	teamId := d.Get("team_id").(string)
	maxBudgetInTeam := d.Get("max_budget_in_team").(float64)

	return TeamMembershipData{
		UserId:          userId,
		Role:            role,
		TeamId:          teamId,
		MaxBudgetInTeam: maxBudgetInTeam,
	}
}

func resourceTeamMembershipCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*LitellmClient)

	var diags diag.Diagnostics

	teamMembershipData := getTeamMembershipData(d)
	apiUrl := fmt.Sprintf("%s/team/member_add", client.ApiBaseURL)

	jsonPayload := map[string]interface{}{
		"member": map[string]string{
			"role":    teamMembershipData.Role,
			"user_id": teamMembershipData.UserId,
		},
		"team_id": teamMembershipData.TeamId,
	}
	if teamMembershipData.MaxBudgetInTeam > 0.0 {
		jsonPayload["max_budget_in_team"] = teamMembershipData.MaxBudgetInTeam
	}
	body, err := json.Marshal(jsonPayload)

	if err != nil {
		diag.FromErr(err)
	}
	req, err := client.NewRequest("POST", apiUrl, bytes.NewBuffer(body))
	if err != nil {
		diag.FromErr(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		diag.Errorf("API request to create team membership has failed with status code %d", resp.StatusCode)
	}

	return diags
}

func resourceTeamMembershipRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*LitellmClient)

	var diags diag.Diagnostics

	teamMembershipData := getTeamMembershipData(d)

	apiUrl := fmt.Sprintf("%s/team/info?team_id=%s", client.ApiBaseURL, teamMembershipData.TeamId)
	req, err := client.NewRequest("GET", apiUrl, nil)
	if err != nil {
		diag.FromErr(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		diag.FromErr(err)
	}
	defer resp.Body.Close()

	var respJsonBody map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&respJsonBody)
	if err != nil {
		diag.FromErr(err)
	}

	jsonTeamMembership, err := json.Marshal(respJsonBody["members_with_roles"])
	if err != nil {
		diag.FromErr(err)
	}
	var membersWithRoles []litellm.MemberWithRole
	err = json.Unmarshal(jsonTeamMembership, membersWithRoles)
	if err != nil {
		diag.FromErr(err)
	}

	for _, teamMembership := range membersWithRoles {
		if teamMembership.UserId == teamMembershipData.UserId {
			d.Set("role", teamMembership.Role)
			d.Set("user_email", teamMembership.UserEmail)
			return nil
		}
	}
	// Reponse .team_memberships.user_id .team_memberships.team_id .team_memberships.budget_id => budget.max_budget

	return diags
}

func resourceTeamMembershipUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*LitellmClient)

	var diags diag.Diagnostics

	teamMembershipData := getTeamMembershipData(d)
	apiUrl := fmt.Sprintf("%s/team/member_update", client.ApiBaseURL)

	jsonPayload := map[string]interface{}{
		"user_id": teamMembershipData.UserId,
		"role":    teamMembershipData.Role,
		"team_id": teamMembershipData.TeamId,
	}
	if teamMembershipData.MaxBudgetInTeam > 0.0 {
		jsonPayload["max_budget_in_team"] = teamMembershipData.MaxBudgetInTeam
	}
	body, err := json.Marshal(jsonPayload)
	if err != nil {
		diag.FromErr(err)
	}
	req, err := client.NewRequest("POST", apiUrl, bytes.NewBuffer(body))
	if err != nil {
		diag.FromErr(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		diag.Errorf("API request to create team membership has failed with status code %d", resp.StatusCode)
	}

	return diags
}

func resourceTeamMembershipDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	client := m.(*LitellmClient)

	var diags diag.Diagnostics

	teamMembershipData := getTeamMembershipData(d)
	apiUrl := fmt.Sprintf("%s/team/member_delete", client.ApiBaseURL)

	jsonPayload, err := json.Marshal(map[string]interface{}{
		"user_id": teamMembershipData.UserId,
		"team_id": teamMembershipData.TeamId,
	})

	if err != nil {
		diag.FromErr(err)
	}
	req, err := client.NewRequest("POST", apiUrl, bytes.NewBuffer(jsonPayload))
	if err != nil {
		diag.FromErr(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		diag.Errorf("API request to create team membership has failed with status code %d", resp.StatusCode)
	}

	return diags
}
