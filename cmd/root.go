package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/Rhypoo-Ma/BOSS-cli/boss"
	"github.com/Rhypoo-Ma/BOSS-cli/browser"
	"github.com/Rhypoo-Ma/BOSS-cli/output"
	"github.com/spf13/cobra"
)

var debug bool

var rootCmd = &cobra.Command{
	Use:   "BOSS-cli",
	Short: "BOSS Zhipin automation CLI",
}

func Execute() error {
	return rootCmd.Execute()
}

func newClient() *browser.Client {
	client := browser.NewClient("BOSS-cli")
	client.Debug = debug
	return client
}

func handleError(code, message string, client *browser.Client) {
	if debug && client != nil {
		if snap, err := client.Snapshot(); err == nil {
			fmt.Fprintf(os.Stderr, "\n--- debug snapshot ---\n%s\n--- end snapshot ---\n", snap)
		}
	}
	output.Error(code, message)
	os.Exit(1)
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Print diagnostic snapshots on errors")

	// login-status
	loginStatusCmd := &cobra.Command{
		Use:   "login-status",
		Short: "Check BOSS login status",
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			status, err := boss.CheckLogin(client)
			if err != nil {
				handleError("login_check_failed", err.Error(), client)
			}
			if !status.LoggedIn {
				handleError("not_logged_in", "Not logged in to BOSS Zhipin. Please open Chrome, navigate to https://www.zhipin.com, and log in manually. Then run this command again.", client)
			}
			output.Success(status)
		},
	}
	rootCmd.AddCommand(loginStatusCmd)

	// list-jobs
	listJobsCmd := &cobra.Command{
		Use:   "list-jobs",
		Short: "List jobs in chat page",
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			jobs, err := boss.ListJobs(client)
			if err != nil {
				handleError("list_jobs_failed", err.Error(), client)
			}
			output.Success(jobs)
		},
	}
	rootCmd.AddCommand(listJobsCmd)

	// switch-job
	switchJobCmd := &cobra.Command{
		Use:   "switch-job [job-name]",
		Short: "Switch to a specific job filter with optional status and unread filters",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			filterStatus, _ := cmd.Flags().GetString("filter")
			unreadOnly, _ := cmd.Flags().GetBool("unread")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			result, err := boss.SwitchJobWithFilters(client, args[0], filterStatus, unreadOnly)
			if err != nil {
				handleError("switch_job_failed", err.Error(), client)
			}
			output.Success(result)
		},
	}
	switchJobCmd.Flags().String("filter", "全部", "Communication status filter: 全部, 新招呼, 沟通中, 已约面, 已获取简历, 已交换电话, 已交换微信, 收藏, 更多")
	switchJobCmd.Flags().Bool("unread", false, "Only show unread messages")
	rootCmd.AddCommand(switchJobCmd)

	// list-candidates
	listCandidatesCmd := &cobra.Command{
		Use:   "list-candidates",
		Short: "List candidates in current job filter",
		Run: func(cmd *cobra.Command, args []string) {
			status, _ := cmd.Flags().GetString("status")
			all, _ := cmd.Flags().GetBool("all")
			max, _ := cmd.Flags().GetInt("max")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			candidates, err := boss.ListCandidates(client, strings.TrimSpace(status), all, max)
			if err != nil {
				handleError("list_candidates_failed", err.Error(), client)
			}
			output.Success(candidates)
		},
	}
	listCandidatesCmd.Flags().String("status", "", "Filter by status: 新招呼, 沟通中, 已约面, 已获取简历, 已交换电话, 已交换微信, 收藏, 更多")
	listCandidatesCmd.Flags().Bool("all", false, "Scroll through the virtual list and load all visible candidates")
	listCandidatesCmd.Flags().Int("max", 0, "Maximum number of candidates to load (0 = unlimited, requires --all)")
	rootCmd.AddCommand(listCandidatesCmd)

	// send-message
	sendMessageCmd := &cobra.Command{
		Use:   "send-message [candidate-name] [message]",
		Short: "Send a message to a candidate",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			message := strings.Join(args[1:], " ")
			if err := boss.SendMessage(client, args[0], message); err != nil {
				handleError("send_message_failed", err.Error(), client)
			}
			output.Success(map[string]string{"candidate": args[0], "sent": "true"})
		},
	}
	rootCmd.AddCommand(sendMessageCmd)

	// download-resume
	downloadResumeCmd := &cobra.Command{
		Use:   "download-resume [candidate-name]",
		Short: "Click to download a candidate's resume",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dir, _ := cmd.Flags().GetString("dir")
			client := newClient()
			_, err := boss.CheckLogin(client)
			if err != nil {
				handleError("not_logged_in", "Please run login-status first and ensure you are logged in.", client)
			}
			if err := boss.DownloadResume(client, args[0], dir); err != nil {
				handleError("download_resume_failed", err.Error(), client)
			}
			output.Success(map[string]string{"candidate": args[0], "downloaded": "true"})
		},
	}
	downloadResumeCmd.Flags().String("dir", "", "Download directory (optional, browser default used)")
	rootCmd.AddCommand(downloadResumeCmd)
}
