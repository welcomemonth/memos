import { WrenchIcon } from "lucide-react";
import MobileHeader from "@/components/MobileHeader";
import useMediaQuery from "@/hooks/useMediaQuery";

const Tools = () => {
  const md = useMediaQuery("md");

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col justify-start items-start rounded-xl bg-background text-foreground overflow-hidden">
          {/* Header */}
          <div className="w-full px-4 py-4 border-b border-border">
            <div className="flex flex-row items-center gap-2">
              <WrenchIcon className="w-5 h-auto text-muted-foreground" />
              <h1 className="text-xl font-semibold">工具集</h1>
            </div>
          </div>

          {/* Content */}
          <div className="w-full px-4 py-6">
            <p className="text-muted-foreground mb-6">这里将放置各种实用小工具。</p>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <div className="rounded-lg border border-border bg-muted/40 px-4 py-6 text-center hover:bg-muted/60 transition-colors cursor-pointer">
                <WrenchIcon className="w-8 h-8 mx-auto mb-3 text-muted-foreground" />
                <h3 className="font-medium text-foreground">待添加</h3>
                <p className="text-sm text-muted-foreground mt-1">更多工具即将上线</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Tools;
