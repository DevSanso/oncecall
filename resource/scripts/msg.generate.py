import json
import re
import sys
import subprocess
from pathlib import Path


ROOT_DIR = Path(__file__).resolve().parents[2]
SRC_DIR = ROOT_DIR / "resource" / "msg"
OUT_DIR = ROOT_DIR / "msg"

MESSAGE_PATTERN = re.compile(
    r'^([NWEC])\s+(\d{5})\s+([A-Za-z_][A-Za-z0-9_]*)\s+("(?:\\.|[^"\\])*")$'
)

FILENAME_PATTERN = re.compile(
    r'^[a-zA-Z][a-zA-Z0-9_]*$'
)

CODE_PATTERN = re.compile(
    r'[a-zA-Z][a-zA-Z0-9_]*_(code|msg)\.go'
)


def get_func_name(name):
    return "".join(
        part[0].upper() + part[1:]
        for part in re.split(r"[_-]", name)
        if part
    )


def parse_msg(path, global_codes, global_names):
    entries = []

    content = path.read_text(encoding="utf-8-sig")

    for line_number, raw_line in enumerate(
        content.splitlines(),
        start=1
    ):
        line = raw_line.strip()

        if not line or line.startswith("#"):
            continue

        match = MESSAGE_PATTERN.fullmatch(line)

        if not match:
            raise ValueError(
                f"{path.name}:{line_number}: Invalid format: {line}"
            )

        level, code_text, name, message_text = match.groups()

        code = int(code_text)

        if code in global_codes:
            raise ValueError(
                f"{path.name}:{line_number}: Duplicate code: {code_text}"
            )

        if name in global_names:
            raise ValueError(
                f"{path.name}:{line_number}: Duplicate constant: {name}"
            )

        try:
            message = json.loads(message_text)
        except json.JSONDecodeError as error:
            raise ValueError(
                f"{path.name}:{line_number}: Invalid message: {error}"
            ) from error

        if not isinstance(message, str):
            raise ValueError(
                f"{path.name}:{line_number}: Message must be a string"
            )

        global_codes.add(code)
        global_names.add(name)

        entries.append({
            "level": level,
            "code": code,
            "name": name,
            "message": message,
        })

    if not entries:
        raise ValueError(f"No messages found: {path.name}")

    return entries


def generate_code(base_idx, base, entries):
    lines = [
        "package msg",
        "",
        f"const {base.upper()}_TAGET MessageTarget = {base_idx}",

        "const (",
    ]

    for entry in entries:
        lines.append(
            f'\t{entry["name"]} MessageCode = {entry["code"]}'
        )

    lines.append(")")

    return "\n".join(lines) + "\n"

"""
	messageMap[0] = map[MessageCode]struct{Level MessageLevel; Name string; Text string} {
		1 : struct{Level MessageLevel; Name string; Text string}{
			Level: "",
			Name: "",
			Text: "",
		},
	}
"""
def generate_message(base, entries):
    target_idx_name = f"{base.upper()}_TAGET"
  
    lines = [
        "package msg",
        "",
        "func init() {", 
        "\tmessageMap[{0}] = map[MessageCode]MessageDesc {{".format(target_idx_name)
    ]

    for entry in entries:
        lines.append(
            f"\t\t{entry["name"]} : {{"
            f'Level: {entry["level"]}, '
            f'Name: {json.dumps(entry["name"])}, '
            f'Text: {json.dumps(entry["message"], ensure_ascii=False)}'
            "},\n"
        )

    lines.append("\t}\n}\n")

    return "\n".join(lines) + "\n"


def remove_stale_files(generated_files):
    for path in OUT_DIR.iterdir():
        if not path.is_file():
            continue

        if CODE_PATTERN.fullmatch(path.name):
            if path not in generated_files:
                path.unlink()
                print(f"Removed: {path.name}")


def main():
    if not SRC_DIR.is_dir():
        raise FileNotFoundError(
            f"MSG directory not found: {SRC_DIR}"
        )

    OUT_DIR.mkdir(parents=True, exist_ok=True)

    files = sorted(SRC_DIR.glob("*.msg"))

    if not files:
        raise FileNotFoundError("No MSG files found")

    global_codes = set()
    global_names = set()
    generated_files = set()

    print("Source encoding: UTF-8")
    base_idx = 0
    for path in files:
        base = path.stem

        if not FILENAME_PATTERN.fullmatch(base):
            raise ValueError(
                f"Invalid filename: {path.name}"
            )

        entries = parse_msg(
            path,
            global_codes,
            global_names
        )

        code_file = OUT_DIR / f"{base}_code.go"
        msg_file = OUT_DIR / f"{base}_msg.go"

        code_file.write_text(
            generate_code(base_idx, base, entries),
            encoding="utf-8",
            newline="\n"
        )
        base_idx += 1

        msg_file.write_text(
            generate_message(base, entries),
            encoding="utf-8",
            newline="\n"
        )

        generated_files.add(code_file)
        generated_files.add(msg_file)

        print(f"Generated: {code_file.name}")
        print(f"Generated: {msg_file.name}")

    remove_stale_files(generated_files)

    subprocess.run(
        [
            "gofmt",
            "-w",
            *map(str, sorted(generated_files))
        ],
        check=True
    )

    print("MSG generation completed successfully.")


if __name__ == "__main__":
    import traceback
    try:
        main()
    except Exception as error:
        print(f"ERROR: {traceback.print_exc()}", file=sys.stderr)
        sys.exit(1)