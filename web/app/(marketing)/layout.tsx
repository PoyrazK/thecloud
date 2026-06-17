
import '../globals.css';

export default function MarketingLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return <div className="marketing-root">{children}</div>;
}
