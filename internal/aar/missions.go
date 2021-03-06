package aar

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

func outputMissionsFromRows(rows pgx.Rows, w http.ResponseWriter) error {
	enc := json.NewEncoder(w)
	w.Write([]byte("["))

	var first = true

	for rows.Next() {
		if first {
			first = false
		} else {
			w.Write([]byte(","))
		}

		mission := Mission{}
		err := rows.Scan(&mission.ID, &mission.CreatedAt, &mission.Length, &mission.Name, &mission.World)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading mission row from database: %v", err)
			continue
		}

		enc.Encode(mission)
	}

	w.Write([]byte("]"))

	return nil
}

func outputMissions(ctx context.Context, w http.ResponseWriter) error {
	rows, err := DB.Query(ctx, `
		SELECT
			id,
			created_at,
			EXTRACT(
				epoch FROM (
					SELECT timestamp
					FROM events
					WHERE events.mission_id = missions.id
					ORDER BY timestamp DESC
					LIMIT 1
				) - (
					SELECT timestamp
					FROM events
					WHERE events.mission_id = missions.id
					ORDER BY timestamp ASC
					LIMIT 1
				)
			)::int AS length,
			name,
			world
		FROM missions
		ORDER BY created_at DESC
	`)

	if err != nil {
		return err
	}
	defer rows.Close()

	return outputMissionsFromRows(rows, w)
}

func outputPlayerMissions(ctx context.Context, playerID string, w http.ResponseWriter) error {
	rows, err := DB.Query(ctx, `
		SELECT
			missions.id,
			missions.created_at,
			EXTRACT(
				epoch FROM (
					SELECT timestamp
					FROM events
					WHERE events.mission_id = missions.id
					ORDER BY timestamp DESC
					LIMIT 1
				) - (
					SELECT timestamp
					FROM events
					WHERE events.mission_id = missions.id
					ORDER BY timestamp ASC
					LIMIT 1
				)
			)::int AS length,
			missions.name,
			missions.world
		FROM events
		INNER JOIN missions ON (missions.id = events.mission_id)
		WHERE events.data #>> '{player, uid}' = $1
		GROUP BY missions.id
		ORDER BY missions.created_at DESC
	`, playerID)

	if err != nil {
		return err
	}
	defer rows.Close()

	return outputMissionsFromRows(rows, w)
}

func outputMission(ctx context.Context, missionID string, w http.ResponseWriter) error {
	row := DB.QueryRow(ctx, `
		SELECT
			id,
			created_at,
			EXTRACT(
				epoch FROM (
					SELECT timestamp
					FROM events
					WHERE events.mission_id = missions.id
					ORDER BY timestamp DESC
					LIMIT 1
				) - (
					SELECT timestamp
					FROM events
					WHERE events.mission_id = missions.id
					ORDER BY timestamp ASC
					LIMIT 1
				)
			)::int AS length,
			name,
			world
		FROM missions
		WHERE id = $1
	`, missionID)
	mission := new(Mission)
	err := row.Scan(&mission.ID, &mission.CreatedAt, &mission.Length, &mission.Name, &mission.World)

	if err != nil {
		return err
	}

	return json.NewEncoder(w).Encode(mission)
}

// MissionsHandler is used to handle the missions endpoint
func MissionsHandler(w http.ResponseWriter, r *http.Request) {
	if err := outputMissions(r.Context(), w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error reading missions: %v", err)
	}
}

// PlayerMissionsHandler is used to handle players endpoint
func PlayerMissionsHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	playerID := params["playerId"]

	if err := outputPlayerMissions(r.Context(), playerID, w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error reading missions: %v", err)
	}
}

// MissionHandler is used to handle the mission endpoint
func MissionHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	missionID := params["missionId"]

	if err := outputMission(r.Context(), missionID, w); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		fmt.Fprintf(os.Stderr, "Error reading mission: %v", err)
	}
}
