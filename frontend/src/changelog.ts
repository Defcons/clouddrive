export const APP_VERSION = '0.0.11'

export interface ChangelogEntry {
  version: string
  date: string
  changes: string[]
}

export const CHANGELOG: ChangelogEntry[] = [
  {
    version: '0.0.11',
    date: '2026-04-03',
    changes: [
      'Added Created column — shows file/folder creation time alongside Modified',
      'Fixed table layout — 4 columns (Name, Size, Created, Modified) with fixed widths that prevent text wrapping',
    ],
  },
  {
    version: '0.0.10',
    date: '2026-04-03',
    changes: [
      'Back/Up buttons now show text labels for better visibility',
      'Fixed Modified column spacing — both Size and Modified columns hug the content',
      'Expand All / Collapse All buttons in sidebar header',
    ],
  },
  {
    version: '0.0.9',
    date: '2026-04-03',
    changes: [
      'Back and Up navigation buttons in breadcrumb bar',
      'Mouse back button (button 4) navigates back in folder history instead of browser',
      'Fixed column spacing — Size and Modified columns sit closer to filenames',
      'Download option now available for both files and folders (folders download as .zip)',
      'Quick Access is now per-user — each user has their own pinned folders',
      'Sidebar refreshes automatically when creating, renaming, or deleting folders',
    ],
  },
  {
    version: '0.0.8',
    date: '2026-04-03',
    changes: [
      'Home folder enforcement — non-admin users are restricted to their home folder',
      'Sidebar loads from home folder, not root — users only see their own folder tree',
      'Breadcrumb respects home folder — no navigation links above home directory',
      'Backend enforces home folder restriction on all file operations',
    ],
  },
  {
    version: '0.0.7',
    date: '2026-04-03',
    changes: [
      'Fix sidebar collapse — folders now collapse on first click without needing to double-click',
      'Folder item count — folders show "X items" instead of empty size',
      'Multi-user auth — users.json config with bcrypt passwords, JWT with role/homeFolder claims',
      'Home folder — each user auto-navigates to their configured home folder on login',
      'Settings modal — view account info and change password (gear icon in header)',
      'Username displayed in header next to version badge',
      'Private folders — right-click a folder to "Make Private" restricting access to you only',
      'Lock icon overlay on private folders in file listing',
      'Permission enforcement on all file operations (list, download, upload, delete, rename, share)',
      'Backward compatible — auto-migrates from env var credentials to users.json',
    ],
  },
  {
    version: '0.0.6',
    date: '2026-04-03',
    changes: [
      'Right-click context menu now works on sidebar folders and Quick Access items',
    ],
  },
  {
    version: '0.0.5',
    date: '2026-04-03',
    changes: [
      'Fix auto-deploy — ensure correct file ownership for deploy user',
    ],
  },
  {
    version: '0.0.4',
    date: '2026-04-03',
    changes: [
      'Fix login — rename USERNAME/PASSWORD env vars to CLOUDDRIVE_USER/CLOUDDRIVE_PASS to avoid system variable conflicts',
    ],
  },
  {
    version: '0.0.3',
    date: '2026-04-03',
    changes: [
      'Move credentials to .env file — secrets no longer stored in git',
      'Added .env.example as template for new deployments',
      'Deploy pipeline auto-creates .env from example if missing',
    ],
  },
  {
    version: '0.0.2',
    date: '2026-04-03',
    changes: [
      'Fix deploy pipeline — force-recreate containers to pick up environment variable changes',
    ],
  },
  {
    version: '0.0.1',
    date: '2026-04-03',
    changes: [
      'Initial release — lightweight file explorer with Go backend and React frontend',
      'File operations: browse, upload, download, rename, delete, create folders',
      'JWT authentication with configurable credentials',
      'List and grid view modes with file type icons',
      'Drag & drop file upload with progress indicator',
      'Right-click context menu for all file operations',
      'File preview for images, PDFs, video, audio, and text files',
      'Share links — public download links with 7-day expiry',
      'Safe Share — password-protected share links',
      'Sidebar with collapsible folder tree hierarchy',
      'Quick Access — pin folders for fast navigation',
      'Breadcrumb path navigation',
      'Docker Compose deployment with Syncthing sidecar for cross-device sync',
      'GitHub Actions auto-deploy on push to master',
      'Deploy notification toast — detects server restarts and prompts refresh',
      'Clickable version badge with changelog modal',
    ],
  },
]
