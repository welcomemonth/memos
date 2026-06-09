import { ArrowLeftRightIcon, WrenchIcon } from "lucide-react";
import { Link } from "react-router-dom";
import MobileHeader from "@/components/MobileHeader";
import useMediaQuery from "@/hooks/useMediaQuery";
import { useTranslate } from "@/utils/i18n";
import { Routes } from "@/router";

const Tools = () => {
  const t = useTranslate();
  const md = useMediaQuery("md");

  // 工具列表（后续添加新工具只需追加到数组）
  const tools = [
    {
      title: t("file-transfer.title"),
      description: t("file-transfer.description"),
      icon: ArrowLeftRightIcon,
      to: Routes.FILE_TRANSFER,
    },
    // {
    //   title: "Base64编解码",
    //   description: "对输入内容进行Base64编码或者解码",
    //   icon: BinaryIcon,
    //   to: Routes.TOOLS,
    // },
  ];

  return (
    <section className="@container w-full max-w-5xl min-h-full flex flex-col justify-start items-center sm:pt-3 md:pt-6 pb-8">
      {!md && <MobileHeader />}
      <div className="w-full px-4 sm:px-6">
        <div className="w-full border border-border flex flex-col justify-start items-start rounded-xl bg-background text-foreground overflow-hidden">
          <div className="w-full px-4 py-4 border-b border-border">
            <div className="flex flex-row items-center gap-2">
              <WrenchIcon className="w-5 h-auto text-muted-foreground" />
              <h1 className="text-xl font-semibold">{t("common.tools")}</h1>
            </div>
          </div>
          <div className="w-full px-4 py-6">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              {tools.map((tool) => (
                <Link
                  key={tool.to}
                  to={tool.to}
                  className="rounded-lg border border-border bg-muted/40 px-4 py-6 text-center hover:bg-muted/60 transition-colors cursor-pointer no-underline"
                >
                  <tool.icon className="w-8 h-8 mx-auto mb-3 text-muted-foreground" />
                  <h3 className="font-medium text-foreground">{tool.title}</h3>
                  <p className="text-sm text-muted-foreground mt-1">{tool.description}</p>
                </Link>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

export default Tools;
