import fs from 'node:fs'
import path from 'node:path'

const [
  latestPath,
  releasesPath,
  version,
  releasedAt,
  packageFileName,
  packageUrl,
  packageSha256,
  packageSizeText,
  releasesUrl
] = process.argv.slice(2)

if (!latestPath || !releasesPath || !version || !releasedAt || !packageFileName || !packageUrl || !packageSha256 || !packageSizeText || !releasesUrl) {
  throw new Error('Missing release manifest arguments')
}

const packageSize = Number(packageSizeText)
if (!Number.isSafeInteger(packageSize) || packageSize <= 0) {
  throw new Error(`Invalid package size: ${packageSizeText}`)
}

const readJson = (filePath) => {
  if (!fs.existsSync(filePath)) return null
  return JSON.parse(fs.readFileSync(filePath, 'utf8').replace(/^\uFEFF/, ''))
}

const normalizeNotes = (value) => {
  if (!Array.isArray(value)) return []
  return value.map((item) => String(item).trim()).filter(Boolean)
}

const parseConfiguredNotes = () => {
  const value = (process.env.AUTO_PRO_RELEASE_NOTES || '').trim()
  if (!value) return []
  if (value.startsWith('[')) return normalizeNotes(JSON.parse(value))
  return normalizeNotes(value.split(/\r?\n/))
}

const previousLatest = readJson(latestPath)
const previousHistory = readJson(releasesPath)
const previousReleases = Array.isArray(previousHistory)
  ? previousHistory
  : Array.isArray(previousHistory?.releases)
    ? previousHistory.releases
    : []

const previousVersion = previousReleases.find((item) => String(item?.version || '').trim() === version)
const configuredNotes = parseConfiguredNotes()
const notes = configuredNotes.length
  ? configuredNotes
  : normalizeNotes(previousVersion?.notes).length
    ? normalizeNotes(previousVersion.notes)
    : String(previousLatest?.version || '').trim() === version && normalizeNotes(previousLatest?.notes).length
      ? normalizeNotes(previousLatest.notes)
      : [`版本 ${version} 更新`]

const channel = (process.env.AUTO_PRO_RELEASE_CHANNEL || 'stable').trim() || 'stable'
const minVersion = (process.env.AUTO_PRO_MIN_VERSION || '0.0.0').trim() || '0.0.0'
const release = { version, channel, releasedAt, notes }

const versionParts = (value) => {
  const matches = String(value).replace(/^v/, '').match(/\d+/g)
  return matches ? matches.map(Number) : []
}

const compareVersionsDesc = (left, right) => {
  const leftParts = versionParts(left.version)
  const rightParts = versionParts(right.version)
  const length = Math.max(leftParts.length, rightParts.length)
  for (let index = 0; index < length; index += 1) {
    const difference = (rightParts[index] || 0) - (leftParts[index] || 0)
    if (difference) return difference
  }
  return String(right.releasedAt || '').localeCompare(String(left.releasedAt || ''))
}

const releases = [
  release,
  ...previousReleases
    .filter((item) => String(item?.version || '').trim() !== version)
    .map((item) => ({
      version: String(item?.version || '').trim(),
      channel: String(item?.channel || 'stable').trim() || 'stable',
      releasedAt: String(item?.releasedAt || '').trim(),
      notes: normalizeNotes(item?.notes)
    }))
    .filter((item) => item.version)
].sort(compareVersionsDesc)

const latest = {
  version,
  channel,
  minVersion,
  force: false,
  releasedAt,
  releasesUrl,
  package: {
    os: 'linux',
    arch: 'amd64',
    fileName: packageFileName,
    url: packageUrl,
    sha256: packageSha256,
    size: packageSize,
    signature: ''
  },
  actions: {
    updateFrontend: true,
    updateBackend: true,
    restartBackend: true,
    backupDatabase: true
  },
  notes
}

const writeJsonAtomic = (filePath, value) => {
  const temporaryPath = `${filePath}.tmp`
  fs.mkdirSync(path.dirname(filePath), { recursive: true })
  fs.writeFileSync(temporaryPath, `${JSON.stringify(value, null, 2)}\n`, 'utf8')
  fs.renameSync(temporaryPath, filePath)
}

writeJsonAtomic(latestPath, latest)
writeJsonAtomic(releasesPath, { releases })