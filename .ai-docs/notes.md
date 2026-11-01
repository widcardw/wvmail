---
description: How the Notes view is put together — one editor per note (why TipTap's setContent makes that necessary), the autosave/flush rules, and how note HTML becomes plain text
---

# Notes

```
views/NotesView.vue            list: filter, select, new, context-menu delete
  └─ components/notes/NoteEditor.vue   :key="note.id" → editor + toolbar + autosave
        └─ stores/notes.ts             loadNotes / createNote / saveNote(id, …) / deleteNote
              └─ NotesService (Go)     notes table (GORM): id, title, content, created_at, updated_at
composables/note-text.ts       pure helpers: title / preview / date (tests/note-text.test.mts)
```

## Rich-text toolbar

`NoteEditor.vue` and `ComposeEditor.vue` share
`components/ui/UiRichTextToolbar.vue` for the editor-level controls:

- text color / background color (`UiColorPicker.vue`), with presets, a native color
  picker, and reset-to-default;
- link editing through an Ark dialog rather than `window.prompt`;
- the same Tiptap `TextStyle`, `Color`, and `BackgroundColor` extensions.

Compose still owns its extra media controls (inline images and attachments), while
Notes owns the “send this note to…” action. The common formatting behavior therefore
does not drift between the two editors.

## One editor per note is load-bearing

`NoteEditor.vue` is mounted with `:key="note.id"`, so every note gets a **fresh TipTap
instance**. Reusing one editor and calling `setContent()` per switch is what caused the
bug this doc exists to prevent:

- In TipTap **v3.22** `setContent(content)` defaults to `emitUpdate: true` **and**
  dispatches an ordinary transaction (`tr.replaceWith(...)`, no `addToHistory: false`).
- The undo history therefore keeps growing across notes. The first Ctrl+Z (or the
  toolbar's undo) after a switch restored the **previous note's document**, and the
  `onUpdate` → autosave path then wrote that mixed text into the current note — i.e.
  silent data corruption, not just a UI glitch.
- There is no public "clear the history" command in the `UndoRedo` extension, so the
  fix is structural: a new instance per note. Fresh history is then empty, and the
  toolbar's undo/redo are `disabled` (`editor.can().undo()`) instead of silently doing
  the wrong thing.

## Autosave rules

| Rule | Why |
|---|---|
| debounce 2000 ms after the last keystroke | cheap, and the note is local |
| save by **explicit id** (`saveNote(id, title, content)`) | the flush runs while the selection has already moved on; keying off `selectedId` wrote into the next note |
| `flush()` on unmount (switch or leaving the view) | the previous version cleared the debounce timer on unmount and **dropped** the last edits |
| `discard()` before deleting the selected note | the unmount flush would otherwise re-insert the deleted note (GORM's `Save` upserts) |
| `restore()` when deletion fails | the note remains open, so a cancelled pending autosave must resume |
| "unsaved" / "saving" indicator in the toolbar | autosave with no feedback looks like nothing was saved |

## Delete flow

Right-clicking a note opens an Ark context menu. Its delete item closes the menu and
opens the shared `ConfirmDialog`; the dialog stays open while the backend request runs
and only closes after success. The selected editor is discarded before the request so
its unmount flush cannot recreate the row. If the request fails, `restore()` puts the
editor back into its normal autosave state and the error is reported with a toast.

## The editor fills the pane

The editable element has to own the pane's height, or the space under the text is not
part of the editor (no caret, no click-to-focus there):

```css
.editor-content-wrapper { flex: 1; min-height: 0; overflow-y: auto; }
.editor-content         { min-height: 100%; display: flex; flex-direction: column; }
.ProseMirror            { flex: 1; min-height: 200px; padding: 20px 24px; cursor: text; }
```

Two details matter: the padding belongs to `.ProseMirror` (padding on the wrapper is a
dead strip), and `min-height: 100%` on the wrapper's child keeps a *long* note scrolling
inside the pane instead of stretching it — with `flex: 1` on that child instead, a long
note can push the pane's height around.

## Note HTML → text

`composables/note-text.ts`, applied in this order:

1. block ends (`</p>`, `<br>`, `</h1…>`, `</li>`, `</blockquote>`, `</pre>`, `</div>`) → a
   separator (`\n` for the title, ` ` for the one-line preview);
2. strip the remaining tags;
3. **then** decode entities (named + numeric).

The order matters: decoding first would turn an escaped `&lt;b&gt;` into a real tag and
delete it. The old inline version stripped tags with one regex over the raw HTML, which
glued blocks together (`<p>a</p><p>b</p>` → `ab`) and left `&amp;`/`&nbsp;` visible.

The stored `title` is derived (first line, 15 chars) because the note list shows a
60-char **preview** instead — the stored title is currently never displayed.

## Known, deliberate limitations

- **No note search in the backend**: the sidebar filter is a client-side match over the
  already-loaded notes (title + full text). Fine for hundreds of local notes.
- `created_at` / `updated_at` are GORM-managed (`autoCreateTime` / `autoUpdateTime`), so
  the timestamps sent by the client are ignored. The store still sets `updated_at`
  locally so the list re-sorts immediately, which is why the sidebar can reorder on each
  save.
- The list is sorted `updated_at DESC` with the note id as a stable tiebreak (several
  saves can land in the same second).
