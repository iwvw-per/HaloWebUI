from datetime import datetime
from pathlib import Path
from typing import Dict, Any, List
from html import escape

import site
from fpdf import FPDF

from open_webui.env import STATIC_DIR, FONTS_DIR
from open_webui.models.chats import ChatTitleMessagesForm


class PDFGenerator:
    """
    Description:
    The `PDFGenerator` class is designed to create PDF documents from chat messages.
    The process involves transforming markdown content into HTML and then into a PDF format

    Attributes:
    - `form_data`: An instance of `ChatTitleMessagesForm` containing title and messages.

    """

    def __init__(self, form_data: ChatTitleMessagesForm):
        self.html_body = None
        self.messages_html = None
        self.form_data = form_data

    def format_timestamp(self, timestamp: float) -> str:
        """Convert a UNIX timestamp to a formatted date string."""
        try:
            date_time = datetime.fromtimestamp(timestamp)
            return date_time.strftime("%Y-%m-%d, %H:%M:%S")
        except (ValueError, TypeError) as e:
            # Log the error if necessary
            return ""

    def _build_html_message(self, message: Dict[str, Any]) -> str:
        """Build HTML for a single message."""
        role = escape(message.get("role", "user"))
        content = escape(message.get("content", ""))
        timestamp = message.get("timestamp")

        model = escape(message.get("model") if role == "assistant" else "")

        date_str = escape(self.format_timestamp(timestamp) if timestamp else "")

        # extends pymdownx extension to convert markdown to html.
        # - https://facelessuser.github.io/pymdown-extensions/usage_notes/
        # html_content = markdown(content, extensions=["pymdownx.extra"])

        content = content.replace("\n", "<br/>")
        html_message = f"""
            <div>
                <div>
                    <h4>
                        <strong>{role.title()}</strong>
                        <span style="font-size: 12px;">{model}</span>
                    </h4>
                    <div> {date_str} </div>
                </div>
                <br/>
                <br/>

                <div>
                    {content}
                </div>
            </div>
            <br/>
          """
        return html_message

    def _generate_html_body(self) -> str:
        """Generate the full HTML body for the PDF."""
        escaped_title = escape(self.form_data.title)
        return f"""
        <html>
            <head>
                <meta name="viewport" content="width=device-width, initial-scale=1.0" />
            </head>
            <body>
            <div>
                <div>
                    <h2>{escaped_title}</h2>
                    {self.messages_html}
                </div>
            </div>
            </body>
        </html>
        """

    def generate_chat_pdf(self) -> bytes:
        """
        Generate a PDF from chat messages.
        """
        try:
            pdf = FPDF()
            pdf.add_page()
            self._configure_fonts(pdf)

            pdf.set_auto_page_break(auto=True, margin=15)

            # Build HTML messages
            messages_html_list: List[str] = [
                self._build_html_message(msg) for msg in self.form_data.messages
            ]
            self.messages_html = "<div>" + "".join(messages_html_list) + "</div>"

            # Generate full HTML body
            self.html_body = self._generate_html_body()

            pdf.write_html(self.html_body)

            # Save the pdf with name .pdf
            pdf_bytes = pdf.output()

            return bytes(pdf_bytes)
        except Exception as e:
            raise e

    def _configure_fonts(self, pdf: FPDF) -> None:
        errors: list[str] = []

        for font_dir in self._iter_font_dirs():
            legacy_error = self._try_configure_legacy_fonts(pdf, font_dir)
            if legacy_error is None:
                return
            errors.append(legacy_error)

            halo_error = self._try_configure_halo_fonts(pdf, font_dir)
            if halo_error is None:
                return
            errors.append(halo_error)

        if errors:
            raise RuntimeError(errors[-1])

        raise RuntimeError(
            "当前服务端缺少可用的 PDF 字体资源，请改用前端页面导出，或在服务端补齐兼容字体文件。"
        )

    def _iter_font_dirs(self) -> list[Path]:
        candidates = [
            Path(FONTS_DIR),
            STATIC_DIR / "fonts",
            STATIC_DIR / "assets" / "fonts",
        ]

        try:
            candidates.append(Path(site.getsitepackages()[0]) / "static" / "fonts")
        except Exception:
            pass

        candidates.append(Path(".") / "backend" / "static" / "fonts")

        unique_candidates: list[Path] = []
        seen: set[str] = set()
        for candidate in candidates:
            candidate_str = str(candidate)
            if candidate_str in seen or not candidate.exists():
                continue
            seen.add(candidate_str)
            unique_candidates.append(candidate)

        return unique_candidates

    def _try_configure_legacy_fonts(self, pdf: FPDF, font_dir: Path) -> str | None:
        regular = font_dir / "NotoSans-Regular.ttf"
        bold = font_dir / "NotoSans-Bold.ttf"
        italic = font_dir / "NotoSans-Italic.ttf"

        if not regular.exists() or not bold.exists():
            return (
                "服务端缺少旧版 PDF 导出字体资源（NotoSans-Regular.ttf / NotoSans-Bold.ttf），"
                "无法继续使用旧的服务端 PDF 栈。"
            )

        try:
            pdf.add_font("NotoSans", "", str(regular))
            pdf.add_font("NotoSans", "b", str(bold))

            if italic.exists():
                pdf.add_font("NotoSans", "i", str(italic))

            fallback_fonts: list[str] = []
            for family, filename in [
                ("NotoSansKR", "NotoSansKR-Regular.ttf"),
                ("NotoSansJP", "NotoSansJP-Regular.ttf"),
                ("NotoSansSC", "NotoSansSC-Regular.ttf"),
                ("Twemoji", "Twemoji.ttf"),
            ]:
                font_path = font_dir / filename
                if not font_path.exists():
                    continue
                pdf.add_font(family, "", str(font_path))
                fallback_fonts.append(family)

            pdf.set_font("NotoSans", size=12)
            if fallback_fonts:
                pdf.set_fallback_fonts(fallback_fonts)

            return None
        except Exception as exc:
            return f"服务端旧版 PDF 字体加载失败：{exc}"

    def _try_configure_halo_fonts(self, pdf: FPDF, font_dir: Path) -> str | None:
        regular = font_dir / "HarmonyOS_SansSC_Regular.woff2"
        bold = font_dir / "HarmonyOS_SansSC_Bold.woff2"

        if not regular.exists():
            return (
                "当前服务端未找到 Halo 自带中文字体，无法使用兼容兜底的服务端 PDF 导出。"
            )

        try:
            pdf.add_font("HaloSansSC", "", str(regular))
            if bold.exists():
                pdf.add_font("HaloSansSC", "b", str(bold))

            pdf.set_font("HaloSansSC", size=12)
            return None
        except Exception as exc:
            return (
                "当前运行环境无法可靠加载 Halo 当前字体（WOFF2），"
                f"服务端 PDF 导出继续中止：{exc}"
            )
