import type { FieldError } from '../../api/model';

export function LocatedErrors({ title, errors }: { title: string; errors: FieldError[] }) {
  if (errors.length === 0) return null;
  return (
    <div role="alert" aria-live="polite">
      <p className="text-xs font-medium text-err">{title}</p>
      <ul className="mt-1 space-y-1 text-xs text-err">
        {errors.map((error, index) => (
          <li key={`${error.field}-${index}`} className="break-words">
            <code className="font-mono">{error.field}</code>: {error.msg}
          </li>
        ))}
      </ul>
    </div>
  );
}
