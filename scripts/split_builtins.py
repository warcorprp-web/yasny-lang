#!/usr/bin/env python3
"""Разрезает interpreter/builtins.go на тематические файлы.

Каждый тематический файл регистрирует свой набор builtin-функций
через init(), добавляя их в глобальную карту `builtins`.
В builtins.go остаются только helper-функции и пустая карта.
"""

import os
import re
import sys

CATEGORIES = {
    "io": ["вывод", "ввод"],
    "files": ["читать_файл", "записать_файл", "существует_файл"],
    "strings": [
        "разделить", "соединить", "заменить", "верхний", "нижний",
        "подстрока", "обрезать", "начинается_с", "заканчивается_на",
        "содержит", "повторить",
    ],
    "numbers": [
        "тип", "строка", "число", "длина", "размер", "округл",
        "мин", "макс", "степень", "корень", "абс", "диапазон",
    ],
    "array": [
        "добавить", "удалить", "вставить", "сортировать", "реверс",
        "найти", "найти_индекс", "все", "любой", "сумма",
        "взять", "пропустить", "преобразовать", "фильтр",
        "дляКаждого", "свернуть", "объединить",
    ],
    "set": ["уникальные", "объединение", "пересечение", "разность"],
    "hash": ["ключи", "значения", "получить"],
    "misc": [
        "ошибка", "загрузить", "установить", "все_ждать",
        "__тест__", "__проверить__",
    ],
}

# Обратное отображение: имя -> категория
NAME_TO_CAT = {}
for cat, names in CATEGORIES.items():
    for n in names:
        NAME_TO_CAT[n] = cat

src = "interpreter/builtins.go"
with open(src, encoding="utf-8") as f:
    lines = f.readlines()

# Находим начало и конец map literal: var builtins = map[string]*Builtin{ ... }
start_idx = None
for i, line in enumerate(lines):
    if line.startswith("var builtins = map[string]*Builtin{"):
        start_idx = i
        break
if start_idx is None:
    sys.exit("не нашёл начало карты builtins")

# Конец — строка с одиночной закрывающей } на нулевом отступе
end_idx = None
depth = 1
for i in range(start_idx + 1, len(lines)):
    line = lines[i]
    # Считаем по фигурным скобкам, игнорируя строки и комментарии
    # (упрощённый учёт, для нашего случая хватает)
    for ch in line:
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                end_idx = i
                break
    if end_idx is not None:
        break

if end_idx is None:
    sys.exit("не нашёл конец карты builtins")

# Извлекаем содержимое карты (без обрамляющих строк)
body = lines[start_idx + 1:end_idx]

# Разбираем на entries: каждая начинается с TAB + "имя": {
ENTRY_RE = re.compile(r'^\t"([^"]+)": \{')

entries_by_cat = {cat: [] for cat in CATEGORIES}
unknown = []

i = 0
while i < len(body):
    line = body[i]
    m = ENTRY_RE.match(line)
    if not m:
        i += 1
        continue
    name = m.group(1)
    # Найти конец entry — строка `\t},` (TAB + }, на отступе 1)
    j = i + 1
    while j < len(body):
        if body[j] == "\t},\n" or body[j].rstrip() == "\t},":
            break
        j += 1
    if j >= len(body):
        sys.exit(f"не нашёл конец entry для {name} начиная с строки {i}")

    # Тело entry — от line[i] до line[j] включительно
    entry_lines = body[i:j + 1]
    # Преобразуем в форму:
    # builtins["name"] = &Builtin{Fn: func(args ...Object) Object { ... }}
    #
    # Текущая форма:
    # \t"name": {
    # \t\tFn: func(args ...Object) Object {
    # \t\t\t...
    # \t\t},
    # \t},
    #
    # Для регистрации в init() нужно:
    # \tbuiltins["name"] = &Builtin{
    # \t\tFn: func(args ...Object) Object {
    # \t\t\t...
    # \t\t},
    # \t}
    #
    # То есть первая строка меняется, последняя теряет одну запятую.
    first = entry_lines[0]
    last = entry_lines[-1]
    new_first = f'\tbuiltins["{name}"] = &Builtin{{\n'
    new_last = "\t}\n"  # без запятой
    transformed = [new_first] + entry_lines[1:-1] + [new_last]

    cat = NAME_TO_CAT.get(name)
    if cat is None:
        unknown.append(name)
        i = j + 1
        continue
    entries_by_cat[cat].append((name, transformed))
    i = j + 1

if unknown:
    sys.exit(f"не классифицированы: {unknown}")

# Готовим тематические файлы
TEMPLATES = {
    "io": "Ввод и вывод",
    "files": "Работа с файлами",
    "strings": "Операции со строками",
    "numbers": "Числа, типы, длина, диапазон",
    "array": "Операции с массивами и функции высшего порядка",
    "set": "Множественные операции на массивах",
    "hash": "Доступ к словарям",
    "misc": "Прочее: ошибки, импорт, тесты, async",
}

for cat, title in TEMPLATES.items():
    path = f"interpreter/builtins_{cat}.go"
    with open(path, "w", encoding="utf-8") as out:
        out.write(f"package interpreter\n\n")
        out.write(f"// {title}.\n\n")
        out.write("func init() {\n")
        for name, body_lines in entries_by_cat[cat]:
            out.write("".join(body_lines))
        out.write("}\n")

# Пишем новый builtins.go: всё до start_idx + новая строка карты + всё после end_idx
new_lines = lines[:start_idx]
new_lines.append("var builtins = make(map[string]*Builtin)\n")
new_lines.extend(lines[end_idx + 1:])

with open(src, "w", encoding="utf-8") as f:
    f.writelines(new_lines)

print("OK: разнесены", sum(len(v) for v in entries_by_cat.values()), "функций")
for cat, items in entries_by_cat.items():
    print(f"  builtins_{cat}.go: {len(items)} (" + ", ".join(n for n, _ in items) + ")")
