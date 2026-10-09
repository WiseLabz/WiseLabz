import { useCallback, useEffect, useMemo, useRef } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { lintGutter, linter, setDiagnostics } from '@codemirror/lint';
import type { Diagnostic } from '@codemirror/lint';
import { EditorView } from '@codemirror/view';

type RecipeDiagnostic = {
  from: number;
  to: number;
  severity: 'error';
  message: string;
  source?: string;
};

export function RecipeYamlEditor({
  value,
  onChange,
  label,
  readOnly,
  diagnostics,
  focusLine,
  focusRequest,
}: {
  value: string;
  onChange: (value: string) => void;
  label: string;
  readOnly: boolean;
  diagnostics: RecipeDiagnostic[];
  focusLine?: number;
  focusRequest: number;
}) {
  const editor = useRef<EditorView | null>(null);
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);
  const handleChange = useCallback((nextValue: string) => onChangeRef.current(nextValue), []);
  const marks = useMemo<Diagnostic[]>(() => diagnostics.map((diagnostic) => ({
    from: diagnostic.from,
    to: diagnostic.to,
    severity: diagnostic.severity,
    message: diagnostic.message,
    source: diagnostic.source,
  })), [diagnostics]);
  const extensions = useMemo(() => [
    yaml(),
    linter(() => marks, { delay: 0 }),
    lintGutter(),
    EditorView.contentAttributes.of({
      id: 'connector-field-recipe',
      'aria-label': label,
      'aria-readonly': String(readOnly),
    }),
  ], [label, marks, readOnly]);
  const basicSetup = useMemo(() => ({
    lineNumbers: true,
    foldGutter: false,
    highlightActiveLine: true,
  }), []);

  useEffect(() => {
    if (editor.current) editor.current.dispatch(setDiagnostics(editor.current.state, marks));
  }, [extensions, marks]);

  useEffect(() => {
    const view = editor.current;
    if (!view || !focusRequest || !focusLine) return;
    const line = view.state.doc.line(Math.max(1, Math.min(view.state.doc.lines, focusLine)));
    view.dispatch({
      selection: { anchor: line.from },
      effects: EditorView.scrollIntoView(line.from, { y: 'center' }),
    });
    view.focus();
  }, [focusLine, focusRequest]);

  return (
    <CodeMirror
      value={value}
      onChange={handleChange}
      extensions={extensions}
      onCreateEditor={(view) => {
        editor.current = view;
        view.dispatch(setDiagnostics(view.state, marks));
      }}
      basicSetup={basicSetup}
      readOnly={readOnly}
      editable={!readOnly}
      className="overflow-hidden rounded-sm border border-line bg-canvas-sunken text-xs"
    />
  );
}
