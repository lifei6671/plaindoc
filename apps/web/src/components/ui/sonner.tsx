import { Toaster as SonnerToaster } from "sonner";

type ToasterProps = React.ComponentProps<typeof SonnerToaster>;

export function Toaster(props: ToasterProps) {
  return (
    <SonnerToaster
      closeButton={false}
      richColors
      position="top-center"
      toastOptions={{
        className: "text-sm",
        duration: 2800
      }}
      {...props}
    />
  );
}
