// Absence of the new field in an older profile means bypass is ON.
export function bypassLocalNetworks(profile: object): boolean {
  return (profile as {routeLocalNetworks?:boolean}).routeLocalNetworks !== true
}

export function withLocalNetworkBypass<T extends object>(profile:T, enabled:boolean): T & {routeLocalNetworks:boolean} {
  return {...profile,routeLocalNetworks:!enabled}
}
