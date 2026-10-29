pylatexenc
==========

Simple LaTeX parser providing latex-to-unicode and unicode-to-latex conversion,
as a Go module.

.. image:: https://img.shields.io/github/license/phfaist/pylatexenc.svg?style=flat
   :target: https://github.com/phfaist/pylatexenc/blob/master/LICENSE.txt

This is a Go port of pylatexenc 2.11. It returns exactly what the Python library
returns, errors included: ``go get github.com/phfaist/pylatexenc`` and import the
packages below. The module has no dependencies outside the Go standard library.


Unicode Text to LaTeX code
--------------------------

The ``latexencode`` package provides ``UnicodeToLatex()``, which converts a
unicode string into LaTeX text and escape sequences. It should recognize
accented characters and most math symbols. The fields of
``latexencode.Options`` alter how it behaves, and an ``UnicodeToLatexEncoder``
can carry custom conversion rules. ``PartialLatexToLatexEncoder`` encodes text
that already contains some LaTeX, keeping the LaTeX characters you name::

    s, err := latexencode.UnicodeToLatex("café – α", latexencode.DefaultOptions())
    // s == `caf\'e {\textendash} \ensuremath{\alpha}`


Parsing LaTeX code & converting to plain text (unicode)
-------------------------------------------------------

The ``latexwalker`` package parses the LaTeX structure of given LaTeX code and
returns a logical structure of nodes, which can then be used to produce output in
another format such as plain text.  This is not a replacement for a full (La)TeX
engine, rather, it provides a way to parse a chunk of LaTeX code as mark-up code.
Its context databases hold the macro, environment and specials definitions that
drive the parser.

The ``latex2text`` package builds up on top of ``latexwalker`` and converts given
LaTeX code to plain text with unicode characters::

    l, _ := latex2text.NewLatexNodes2Text(latex2text.DefaultOptions())
    text, err := l.LatexToText(`\'{e}t\'{e} \textbf{caf\"e}`, latexwalker.DefaultOptions())
    // text == "été cafë"


Documentation
-------------

The documentation of the Python library, in ``doc/`` and at
https://pylatexenc.readthedocs.io/, describes the behaviour; the Go packages keep
its names, with methods and options in Go's spelling.


License
-------

See LICENSE.txt (MIT License).

NOTE: See copyright notice and license information for file
``tools/unicode.xml`` provided in ``tools/unicode.xml.LICENSE``.  (The file
``tools/unicode.xml`` was downloaded from
https://www.w3.org/2003/entities/2007xml/unicode.xml as linked from
https://www.w3.org/TR/xml-entity-names/#source.)
