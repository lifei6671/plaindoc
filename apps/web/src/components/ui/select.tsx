import * as React from "react";
import { ChevronDown } from "lucide-react";
import { cn } from "../../lib/utils";

type SelectOption = {
  value: string;
  label: React.ReactNode;
  disabled?: boolean;
};

type SelectContextValue = {
  disabled?: boolean;
  options: SelectOption[];
  setOptions: (options: SelectOption[]) => void;
  value: string;
  setValue: (value: string) => void;
};

const SelectContext = React.createContext<SelectContextValue | null>(null);

function useSelectContext(componentName: string): SelectContextValue {
  const context = React.useContext(SelectContext);
  if (context == null) {
    throw new Error(`${componentName} must be used within Select`);
  }
  return context;
}

function extractSelectPlaceholder(children: React.ReactNode): string {
  let placeholder = "";
  React.Children.forEach(children, (child) => {
    if (!React.isValidElement<{ placeholder?: string }>(child)) {
      return;
    }
    if (child.type === SelectValue) {
      const valuePlaceholder = child.props.placeholder;
      if (typeof valuePlaceholder === "string") {
        placeholder = valuePlaceholder;
      }
    }
  });
  return placeholder;
}

function flattenSelectOptions(children: React.ReactNode): SelectOption[] {
  const options: SelectOption[] = [];

  const visit = (nodeChildren: React.ReactNode) => {
    React.Children.forEach(nodeChildren, (child) => {
      if (!React.isValidElement<{ value?: string; disabled?: boolean; children?: React.ReactNode }>(child)) {
        return;
      }
      if (child.type === SelectItem) {
        const value = typeof child.props.value === "string" ? child.props.value : "";
        if (!value) {
          return;
        }
        options.push({
          value,
          label: child.props.children,
          disabled: Boolean(child.props.disabled)
        });
        return;
      }
      if (child.type === SelectGroup || child.type === React.Fragment) {
        visit(child.props.children);
      }
    });
  };

  visit(children);
  return options;
}

type SelectProps = {
  value?: string;
  defaultValue?: string;
  onValueChange?: (value: string) => void;
  disabled?: boolean;
  children?: React.ReactNode;
};

function Select({ value, defaultValue, onValueChange, disabled, children }: SelectProps) {
  const [internalValue, setInternalValue] = React.useState(defaultValue ?? "");
  const [options, setOptions] = React.useState<SelectOption[]>([]);
  const isControlled = value !== undefined;
  const currentValue = isControlled ? value ?? "" : internalValue;

  const handleValueChange = React.useCallback(
    (nextValue: string) => {
      if (!isControlled) {
        setInternalValue(nextValue);
      }
      onValueChange?.(nextValue);
    },
    [isControlled, onValueChange]
  );

  const contextValue = React.useMemo<SelectContextValue>(
    () => ({
      disabled,
      options,
      setOptions,
      value: currentValue,
      setValue: handleValueChange
    }),
    [currentValue, disabled, handleValueChange, options]
  );

  return <SelectContext.Provider value={contextValue}>{children}</SelectContext.Provider>;
}

type SelectTriggerProps = React.SelectHTMLAttributes<HTMLSelectElement> & {
  children?: React.ReactNode;
};

const SelectTrigger = React.forwardRef<HTMLSelectElement, SelectTriggerProps>(
  ({ className, children, onChange, ...props }, ref) => {
    const context = useSelectContext("SelectTrigger");
    const placeholder = extractSelectPlaceholder(children);
    const hasSelectedOption = context.options.some((option) => option.value === context.value);
    const selectValue = hasSelectedOption ? context.value : "";

    return (
      <div className="relative">
        <select
          ref={ref}
          className={cn(
            "flex h-9 w-full appearance-none items-center justify-between rounded-md border border-slate-300 bg-white px-3 py-1 pr-9 text-sm shadow-sm ring-offset-white focus:outline-none focus:ring-2 focus:ring-slate-400 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50",
            className
          )}
          value={selectValue}
          disabled={context.disabled || props.disabled}
          onChange={(event) => {
            context.setValue(event.target.value);
            onChange?.(event);
          }}
          {...props}
        >
          {placeholder ? (
            <option value="" disabled>
              {placeholder}
            </option>
          ) : null}
          {context.options.map((option) => (
            <option key={option.value} value={option.value} disabled={option.disabled}>
              {typeof option.label === "string" || typeof option.label === "number" ? option.label : String(option.value)}
            </option>
          ))}
        </select>
        <ChevronDown className="pointer-events-none absolute right-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-500" />
      </div>
    );
  }
);
SelectTrigger.displayName = "SelectTrigger";

type SelectValueProps = {
  placeholder?: string;
  children?: React.ReactNode;
};

function SelectValue(_props: SelectValueProps) {
  return null;
}

type SelectContentProps = {
  children?: React.ReactNode;
  className?: string;
  position?: "popper" | "item-aligned";
};

const SelectContent = React.forwardRef<HTMLDivElement, SelectContentProps>(({ children }, _ref) => {
  const context = useSelectContext("SelectContent");
  const options = React.useMemo(() => flattenSelectOptions(children), [children]);

  React.useLayoutEffect(() => {
    context.setOptions(options);
  }, [context, options]);

  return null;
});
SelectContent.displayName = "SelectContent";

type SelectItemProps = {
  value: string;
  disabled?: boolean;
  className?: string;
  children?: React.ReactNode;
};

function SelectItem(_props: SelectItemProps) {
  return null;
}

type SelectGroupProps = {
  children?: React.ReactNode;
};

function SelectGroup({ children }: SelectGroupProps) {
  return <>{children}</>;
}

type SelectLabelProps = {
  children?: React.ReactNode;
  className?: string;
};

function SelectLabel(_props: SelectLabelProps) {
  return null;
}

const SelectScrollUpButton = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>((_props, _ref) => null);
SelectScrollUpButton.displayName = "SelectScrollUpButton";

const SelectScrollDownButton = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>((_props, _ref) => null);
SelectScrollDownButton.displayName = "SelectScrollDownButton";

const SelectSeparator = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>((_props, _ref) => null);
SelectSeparator.displayName = "SelectSeparator";

export {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectScrollDownButton,
  SelectScrollUpButton,
  SelectSeparator,
  SelectTrigger,
  SelectValue
};
