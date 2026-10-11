import { callApi } from "@/lib/api/server";
import type { components, operations } from "@/lib/api/schema";

export type Location = components["schemas"]["Location"];
export type CreateLocationRequest = components["schemas"]["CreateLocationRequest"];
export type UpdateLocationRequest = components["schemas"]["UpdateLocationRequest"];
export type Room = components["schemas"]["Room"];
export type CreateRoomRequest = components["schemas"]["CreateRoomRequest"];
export type UpdateRoomRequest = components["schemas"]["UpdateRoomRequest"];

type Ok<K extends keyof operations, S extends number> = operations[K] extends {
  responses: Record<S, { content: { "application/json": infer T } }>;
}
  ? T
  : never;

export function listLocations(includeArchived = false) {
  const query = includeArchived ? "?includeArchived=true" : "";

  return callApi<Ok<"listLocations", 200>>(`/api/v1/locations${query}`);
}

export function createLocation(body: CreateLocationRequest) {
  return callApi<Ok<"createLocation", 201>>("/api/v1/locations", { method: "POST", body });
}

export function getLocation(locationId: string) {
  return callApi<Ok<"getLocation", 200>>(`/api/v1/locations/${encodeURIComponent(locationId)}`);
}

export function updateLocation(locationId: string, body: UpdateLocationRequest) {
  return callApi<Ok<"updateLocation", 200>>(`/api/v1/locations/${encodeURIComponent(locationId)}`, {
    method: "PATCH",
    body,
  });
}

export function archiveLocation(locationId: string) {
  return callApi<void>(`/api/v1/locations/${encodeURIComponent(locationId)}`, { method: "DELETE" });
}

export function listRooms(locationId: string, includeArchived = false) {
  const query = includeArchived ? "?includeArchived=true" : "";

  return callApi<Ok<"listRooms", 200>>(
    `/api/v1/locations/${encodeURIComponent(locationId)}/rooms${query}`,
  );
}

export function createRoom(locationId: string, body: CreateRoomRequest) {
  return callApi<Ok<"createRoom", 201>>(
    `/api/v1/locations/${encodeURIComponent(locationId)}/rooms`,
    {
      method: "POST",
      body,
    },
  );
}

export function getRoom(roomId: string) {
  return callApi<Ok<"getRoom", 200>>(`/api/v1/rooms/${encodeURIComponent(roomId)}`);
}

export function archiveRoom(roomId: string) {
  return callApi<void>(`/api/v1/rooms/${encodeURIComponent(roomId)}`, { method: "DELETE" });
}
