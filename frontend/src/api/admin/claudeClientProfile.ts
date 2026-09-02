import { apiClient } from '../client'

export interface ClaudeClientProfile {
  id: string
  name: string
  description: string
  cli_version: string
  sdk_package_version: string
  default_tls_mode: string
  beta_names?: string[]
  supports_request_gzip: boolean
  gzip_min_chars?: number
}

export async function list(): Promise<ClaudeClientProfile[]> {
  const { data } = await apiClient.get<ClaudeClientProfile[]>('/admin/claude-client-profiles')
  return data
}

export const claudeClientProfileAPI = {
  list
}

export default claudeClientProfileAPI
