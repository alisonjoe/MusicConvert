# Ticket 03: WebUI Frontend Components & User Interaction

- **Status**: Ready
- **Blocked by**: #2
- **Module**: `crates/music-convert/src/server/assets.rs`

## Description
Enhance the single-page Vue 3 Web console with the 3 requested features:
1. Source folder selector & directory browser modal.
2. Converted library view with search, pagination, and reconvert action.
3. Top bar "⚡ 立即手动扫描" manual convert trigger button.

## Acceptance Criteria
1. Top bar contains:
   - Current source path badge with "📂 更改目录" button.
   - "⚡ 立即手动扫描" button with loading indicator and toast notification.
2. Directory browser modal:
   - Allows navigating down directories (e.g. `/`, `/volume1`, etc.), displaying subdirectories.
   - Allows typing arbitrary path and testing/saving as active source folder.
3. Converted Library Tab:
   - Displays list of converted music with total counts, pagination (Next / Prev), and instant search.
   - Each row has a "🔄 重新转换" button which prompts confirmation and calls `/api/converted/reconvert`, then notifies the user.
4. Assets are cleanly embedded via `rust-embed` and compile cleanly with `cargo test --workspace`.
