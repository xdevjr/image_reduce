---
name: Image Reduce
description: Estação de processamento de imagens para Linux
colors:
  void-navy: "#0f1117"
  deep-slate: "#171a23"
  panel-grey: "#1f2430"
  hairline: "#2a3040"
  pearl: "#e6e9f0"
  fog: "#8b93a7"
  arc-blue: "#4f8cff"
  grow-green: "#46c17a"
  ember-red: "#ff5c5c"
  amber: "#f0b429"
typography:
  display:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, sans-serif"
    fontSize: "16px"
    fontWeight: 700
    letterSpacing: "0.3px"
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, sans-serif"
    fontSize: "13px"
    fontWeight: 400
  label:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, sans-serif"
    fontSize: "12px"
    fontWeight: 600
  caption:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, sans-serif"
    fontSize: "11px"
rounded:
  sm: "6px"
  md: "8px"
  lg: "10px"
  pill: "20px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "14px"
  lg: "20px"
components:
  button-primary:
    backgroundColor: "{colors.arc-blue}"
    textColor: "#ffffff"
    rounded: "{rounded.md}"
    padding: "11px"
  button-ghost:
    backgroundColor: "{colors.panel-grey}"
    textColor: "{colors.pearl}"
    rounded: "{rounded.md}"
    padding: "8px 14px"
  input-text:
    backgroundColor: "{colors.panel-grey}"
    textColor: "{colors.pearl}"
    rounded: "{rounded.md}"
    padding: "10px 12px"
  card-row:
    backgroundColor: "{colors.deep-slate}"
    textColor: "{colors.pearl}"
    rounded: "{rounded.lg}"
    padding: "10px 14px"
---

# Design System: Image Reduce

## Overview

**Creative North Star: "The Darkroom Console"**

O Image Reduce é um laboratório escuro e silencioso onde imagens brutas são processadas e enviadas: uma estação de processamento para Linux que monitora uma pasta e converte em WebP/WebM sem pedir atenção. A interface é um instrumento de precisão — densa o bastante para ser útil, limpa o bastante para não cansar. Não há decoração, marca ou entusiasmo visual: só um painel escuro, calmo e factual que informa o que está acontecendo e deixa o usuário ajustar o pipeline.

A cor é tratada como semáforo, não como identidade. O canvas permanece neutro em três camadas tonais (Void Navy → Deep Slate → Panel Grey), e cada matiz saturado significa exatamente uma coisa: azul para ação/processando, verde para pronto, âmbar para aviso/pausado, vermelho para erro. A tipografia é nativa do sistema e compacta (11–16px), reforçando o caráter de ferramenta utilitária em vez de site. Profundidade é expressa por camadas de superfície, não por sombras — as superfícies são planas em repouso.

**Key Characteristics:**

- Escuro por padrão: três camadas de superfície tonal (Void Navy → Deep Slate → Panel Grey).
- Cor é sinal, não decoração: Arc Blue, Grow Green, Amber e Ember Red carregam significado.
- Controles quietos e compactos (tipo de 11–16px), hierarquia por peso e tamanho.
- Flat em repouso, profundidade por camadas tonais; sombra sutil apenas em estado interativo.
- Layout de painel fixo em webview de coluna única, sem breakpoints responsivos.

## Colors

Uma paleta fria e contida sobre um azul-quase-preto; o único acento saturado "de ação" é o azul, e as matizes de status aparecem apenas como texto de badge sobre fundos tintados escuros.

### Primary

- **Arc Blue** (#4f8cff): o único acento interativo. Fundo do botão primário, borda de hover/foco em inputs e botões ghost, sublinhado e texto da aba ativa, texto do badge "converting" e do arquivo em conversão. Nunca é usado como decoração.

### Status (semântico)

- **Grow Green** (#46c17a): sucesso. Texto do badge "done" e do nome de arquivo em linhas concluídas.
- **Ember Red** (#ff5c5c): erro/destrutivo. Badge "error", nome de arquivo em linhas com erro, hover destrutivo (Limpar histórico, botão de fechar), borda/texto de toast de erro.
- **Amber** (#f0b429): aviso/pausado. Badge "skipped" e estado do botão de pausar/retomar monitoramento.

### Neutral

- **Void Navy** (#0f1117): fundo do aplicativo, camada mais profunda.
- **Deep Slate** (#171a23): superfície — cabeçalho e linhas do histórico.
- **Panel Grey** (#1f2430): painel elevado — inputs, botões ghost, toast.
- **Hairline** (#2a3040): bordas de 1px e divisores; também fundo do badge "queued".
- **Pearl** (#e6e9f0): texto primário.
- **Fog** (#8b93a7): texto secundário, placeholders e leituras de status.

### Named Rules

**The Signal Color Rule.** Acento e cores de status carregam significado; o fundo neutro permanece sem cor. Uma linha do histórico nunca é tintada arbitrariamente — apenas o badge e o nome do arquivo adotam a matiz de status.

**The Tint-Pair Rule.** Cada matiz de status aparece em um fundo de badge escuro e tintado emparelhado com seu texto saturado: converting (#22304d + Arc Blue), done (#14331f + Grow Green), skipped (#3a2f12 + Amber), error (#3a1616 + Ember Red). Nunca troque o par.

## Typography

**Body Font:** pilha do sistema (-apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif)

**Character:** nativa, quieta, utilitária — a interface usa a fonte do sistema em tamanhos compactos para que a ferramenta leia como um instrumento, não como uma declaração de marca. A hierarquia vem do peso e do tamanho, nunca da cor sozinha.

### Hierarchy

- **Display** (700, 16px, letter-spacing 0.3px): título "Image Reduce" no cabeçalho; o acento está na segunda palavra.
- **Nav** (400→600, 13px): rótulos das abas; a aba ativa usa acento + sublinhado de 2px.
- **Body** (400, 13px): linhas do histórico, valores de input, rótulos de botão.
- **Label** (600, 12px): rótulos de campos do formulário.
- **Caption** (400, 11–12px, muted Fog): leitura de status, timestamps, dicas e textos de erro.
- **Badge** (600, 11px, uppercase, letter-spacing 0.4px): pílulas de status.

### Named Rules

**The Small-Type Instrument Rule.** Nada no chrome ultrapassa 16px. Precisão vem do tipo compacto e do ritmo de espaçamento, não do tamanho.

## Layout

Shell de aplicativo em coluna única (flex column): cabeçalho (padding 14px 20px, borda inferior Hairline de 1px) → nav de abas (padding 10px 20px 0, gap 4px) → main rolável (padding 16px 20px). A lista de histórico é uma pilha vertical de linhas com gap de 8px. O formulário de configuração é limitado a 520px, com campos empilhados a 14px; pares de campos numéricos relacionados ficam lado a lado (row-fields, gap 14px). A janela é um webview de tamanho fixo — sem breakpoints responsivos; o conteúdo rola dentro do main.

## Elevation & Depth

Flat por padrão: a profundidade é transmitida com três camadas de superfície tonal (Void Navy → Deep Slate → Panel Grey) mais bordas Hairline de 1px, não com sombras. Por decisão de design, uma sombra sutil pode elevar elementos interativos apenas em hover/focus; as superfícies são planas em repouso.

### Named Rules

**The Flat-By-Default Rule.** Sem sombras em repouso. A elevação é expressa por camadas de superfícies neutras; uma sombra de estado aparece apenas como resposta a hover ou foco.

## Shapes

Sistema de dois raios com exceção de pílulas: controles e botões ghost usam 8px (md), containers e linhas do histórico usam 10px (lg), o botão de fechar 6px (sm), e badges de status são pílulas completas (20px). As bordas são sempre de 1px Hairline. Não há clipping nem outra geometria recorrente.

## Components

### Buttons

- **Shape:** cantos suavemente arredondados (raio md de 8px).
- **Primary (Salvar configurações):** fundo Arc Blue, texto branco, largura total, padding 11px, sem borda. Hover: `brightness(1.1)` (filter, 0.15s).
- **Ghost (Abrir pasta, Selecionar…, botões de ação):** fundo Panel Grey, borda Hairline de 1px, texto Pearl, padding 8px 14px. Hover: borda → Arc Blue, fundo → tint Arc Blue (#22304d).
- **Danger hover (Limpar histórico, botão fechar):** borda → Ember Red, fundo → tint Ember Red (#3a1616).
- **Paused (Pausar/Retomar monitoramento):** borda + texto → Amber, rótulo alterna entre "⏸ Pausar" / "▶ Retomar".

### Chips (badges de status)

- **Style:** pílula (raio 20px), uppercase 11px/600, tracking 0.4px, padding 3px 8px, fundo tintado escuro + texto saturado conforme o status (The Tint-Pair Rule).

### Cards / Containers (linhas do histórico)

- **Corner Style:** 10px (lg).
- **Background:** Deep Slate.
- **Border:** 1px Hairline.
- **Shadow Strategy:** nenhuma (flat; ver Elevation).
- **Internal Padding:** 10px 14px; gap interno 10px.

### Inputs / Fields

- **Style:** fundo Panel Grey, borda Hairline de 1px, raio 8px, padding 10px 12px, texto 13px.
- **Focus:** borda → Arc Blue (transição `border-color 0.15s`).
- **Error / Disabled:** texto de erro é renderizado inline como `.err` em Ember Red; não há borda de erro em nível de campo atualmente.

### Navigation (abas)

- **Style:** abas apenas de texto, 13px, Fog em repouso; hover → Pearl; ativa → texto Arc Blue + sublinhado Arc Blue de 2px (`border-bottom`), raio 8px apenas nos cantos superiores.

### Componente de assinatura: History Row

Uma linha conta toda a história de status: badge em pílula (status) → nome do arquivo (peso 600, tintado conforme status em linhas converting/done/error) → delta de tamanho "1.2 MB → 400 KB (-67%)" → caption de motivo/erro → timestamp. O nome do arquivo trunca com reticências; a linha nunca quebra.

### Componente de assinatura: Toast

Fixado no centro inferior, superfície Panel Grey, borda Hairline, raio 8px, entra via `translateY(80px→0)` + fade (0.25s), desaparece sozinho após 2600ms. Toasts de erro trocam borda + texto para Ember Red.

## Do's and Don'ts

### Do

- **Do** manter o canvas neutro (Void Navy / Deep Slate / Panel Grey) e reservar Arc Blue para elementos interativos ou de sinal.
- **Do** emparelhar cada cor de status com seu fundo tintado escuro (The Tint-Pair Rule).
- **Do** usar raio 8px para controles e 10px para containers; badges permanecem pílulas.
- **Do** manter o tipo do chrome ≤16px e deixar peso/tamanho — não cor — carregarem a hierarquia.
- **Do** comunicar estado com rótulo + cor juntos (o texto do badge sempre acompanha a tinta).

### Don't

- **Don't** adicionar sombras de projeção em repouso — elevação é camada tonal; sombras só em estado interativo.
- **Don't** introduzir matizes fora do conjunto de tokens (sem roxo, laranja, rosa) — a paleta é um sistema fechado.
- **Don't** tintar superfícies inteiras com cores de status; apenas o badge e o nome do arquivo adotam a matiz.
- **Don't** deixar o formulário de configuração ultrapassar 520px nem empilhar os pares de duas colunas neste webview fixo.
