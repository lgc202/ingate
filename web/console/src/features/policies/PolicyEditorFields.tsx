import { useId } from 'react';

export function PolicyInputField({
  label,
  value,
  placeholder,
  type = 'text',
  onChange,
}: {
  label: string;
  value: string;
  placeholder?: string;
  type?: string;
  onChange: (value: string) => void;
}) {
  const inputID = useId();
  return (
    <div className="field">
      <label htmlFor={inputID}>{label}</label>
      <input id={inputID} value={value} type={type} placeholder={placeholder} onChange={(event) => onChange(event.target.value)} />
    </div>
  );
}

export function PolicySelectField({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: Array<[string, string]>;
  onChange: (value: string) => void;
}) {
  const selectID = useId();
  return (
    <div className="field">
      <label htmlFor={selectID}>{label}</label>
      <select id={selectID} value={value} onChange={(event) => onChange(event.target.value)}>
        {options.map(([optionValue, labelText]) => (
          <option key={optionValue || labelText} value={optionValue}>{labelText}</option>
        ))}
      </select>
    </div>
  );
}
