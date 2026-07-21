import { useId } from 'react';

export function PolicyInputField({
  label,
  value,
  placeholder,
  type = 'text',
  error,
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  type?: string;
  error?: string;
  onChange: (value: string) => void;
}) {
  const inputID = useId();
  const errorID = `${inputID}-error`;
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <label htmlFor={inputID}>{label}</label>
      <input
        id={inputID}
        value={value}
        type={type}
        placeholder={placeholder}
        aria-invalid={Boolean(error)}
        aria-describedby={error ? errorID : undefined}
        onChange={(event) => onChange(event.target.value)}
      />
      {error ? <div id={errorID} className="form-error" role="alert">{error}</div> : null}
    </div>
  );
}

export function PolicySelectField({
  label,
  value,
  options,
  error,
  onChange,
}: {
  label: string;
  value: string;
  options: Array<[string, string]>;
  error?: string;
  onChange: (value: string) => void;
}) {
  const selectID = useId();
  const errorID = `${selectID}-error`;
  return (
    <div className={`field ${error ? 'invalid' : ''}`.trim()}>
      <label htmlFor={selectID}>{label}</label>
      <select
        id={selectID}
        value={value}
        aria-invalid={Boolean(error)}
        aria-describedby={error ? errorID : undefined}
        onChange={(event) => onChange(event.target.value)}
      >
        {options.map(([optionValue, labelText]) => (
          <option key={optionValue || labelText} value={optionValue}>{labelText}</option>
        ))}
      </select>
      {error ? <div id={errorID} className="form-error" role="alert">{error}</div> : null}
    </div>
  );
}
