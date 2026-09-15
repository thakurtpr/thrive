//go:build linux

package commands

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/thakurprasadrout/thrive/internal/events"
	thrsystem "github.com/thakurprasadrout/thrive/internal/system"
)

func systemDfCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "df",
		Short: "Show disk usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			u, err := thrsystem.DiskUsage()
			if err != nil {
				return err
			}
			fmt.Printf("%-12s %-7s %-7s %-10s %s\n", "TYPE", "TOTAL", "ACTIVE", "SIZE", "RECLAIMABLE")
			printUsageRow("Images", u.Images)
			printUsageRow("Containers", u.Containers)
			printUsageRow("Volumes", u.Volumes)
			printUsageRow("Networks", u.Networks)
			fmt.Printf("%-12s %-7d %-7s %-10s %s\n",
				"Build Cache", 0, "-", humanBytes(u.BuildCache.Size), humanBytes(u.BuildCache.Reclaimable))
			return nil
		},
	}
}

func printUsageRow(name string, e thrsystem.UsageEntry) {
	fmt.Printf("%-12s %-7d %-7d %-10s %s\n",
		name, e.Total, e.Active, humanBytes(e.Size), humanBytes(e.Reclaimable))
}

func humanBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func systemEventsCmd() *cobra.Command {
	var sinceStr, untilStr, filterType, format string
	var follow bool
	cmd := &cobra.Command{
		Use:   "events",
		Short: "Stream engine events",
		RunE: func(cmd *cobra.Command, args []string) error {
			filter, err := parseEventsFilter(sinceStr, untilStr, filterType)
			if err != nil {
				return err
			}
			list, err := events.Query(filter)
			if err != nil {
				return err
			}
			for _, e := range list {
				printEvent(e, format)
			}
			if !follow {
				return nil
			}
			seen := len(list)
			for {
				time.Sleep(500 * time.Millisecond)
				list, err := events.Query(filter)
				if err != nil {
					return err
				}
				for ; seen < len(list); seen++ {
					printEvent(list[seen], format)
				}
			}
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "", "Show events since timestamp or duration (e.g. 10m, 2026-09-15T12:00:00Z)")
	cmd.Flags().StringVar(&untilStr, "until", "", "Show events until timestamp or duration")
	cmd.Flags().StringVar(&filterType, "filter", "", "Filter by type (container, image, network, volume)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow event output")
	cmd.Flags().StringVar(&format, "format", "", "Output format (json for raw JSON lines)")
	return cmd
}

func printEvent(e events.Event, format string) {
	if format == "json" {
		fmt.Println(events.FormatJSON(e))
		return
	}
	fmt.Println(events.Format(e))
}

func parseEventsFilter(sinceStr, untilStr, filterType string) (events.Filter, error) {
	var f events.Filter
	now := time.Now().UTC()
	if sinceStr != "" {
		t, err := parseEventTime(sinceStr, now)
		if err != nil {
			return f, fmt.Errorf("invalid --since: %w", err)
		}
		f.Since = t
	}
	if untilStr != "" {
		t, err := parseEventTime(untilStr, now)
		if err != nil {
			return f, fmt.Errorf("invalid --until: %w", err)
		}
		f.Until = t
	}
	if filterType != "" {
		// Accept "type=container" or bare "container".
		f.Type = filterType
		for _, prefix := range []string{"type=", "event="} {
			if len(filterType) > len(prefix) && filterType[:len(prefix)] == prefix {
				f.Type = filterType[len(prefix):]
			}
		}
	}
	return f, nil
}

func parseEventTime(s string, now time.Time) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return now.Add(-d), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0).UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q (use 10m, RFC3339, or unix timestamp)", s)
}

func systemPruneCmd() *cobra.Command {
	var allImages, volumes, force bool
	cmd := &cobra.Command{
		Use:   "prune",
		Short: "Remove unused data (stopped containers, networks, optionally images and volumes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !force {
				fmt.Print("This will remove stopped containers, unused networks")
				if allImages {
					fmt.Print(", unreferenced images")
				}
				if volumes {
					fmt.Print(", unused volumes")
				}
				fmt.Print(". Continue? [y/N] ")
				var answer string
				_, _ = fmt.Scanln(&answer)
				if answer != "y" && answer != "Y" {
					fmt.Println("Cancelled.")
					return nil
				}
			}
			rep, err := thrsystem.Prune(thrsystem.PruneOptions{AllImages: allImages, Volumes: volumes})
			if err != nil {
				return err
			}
			fmt.Printf("Deleted containers: %d\n", rep.ContainersDeleted)
			fmt.Printf("Deleted images: %d\n", rep.ImagesDeleted)
			fmt.Printf("Deleted volumes: %d\n", rep.VolumesDeleted)
			fmt.Printf("Deleted networks: %d\n", rep.NetworksDeleted)
			fmt.Printf("Total reclaimed space: %s\n", humanBytes(rep.SpaceReclaimed))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&allImages, "all", "a", false, "Remove all unreferenced images")
	cmd.Flags().BoolVar(&volumes, "volumes", false, "Also prune unused volumes")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Do not prompt for confirmation")
	return cmd
}
