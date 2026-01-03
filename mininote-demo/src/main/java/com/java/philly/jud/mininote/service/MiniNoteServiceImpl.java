package com.java.philly.jud.mininote.service;

import com.java.philly.jud.mininote.configuration.MiniNoteProperties;
import com.java.philly.jud.mininote.model.MiniNote;
import com.java.philly.jud.mininote.repository.MiniNoteRepository;
import lombok.SneakyThrows;
import org.commonmark.node.Node;
import org.commonmark.parser.Parser;
import org.commonmark.renderer.html.HtmlRenderer;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.ui.Model;
import org.springframework.web.multipart.MultipartFile;

import java.io.File;
import java.util.List;
import java.util.Optional;
import java.util.UUID;

@Service
public class MiniNoteServiceImpl implements IMiniNoteService{

    private MiniNoteRepository miniNotesRepository;

    private MiniNoteProperties properties;

    private Parser parser;
    private HtmlRenderer renderer;

    @Autowired
    public MiniNoteServiceImpl(MiniNoteRepository miniNotesRepository, MiniNoteProperties properties) {
        this.miniNotesRepository = miniNotesRepository;
        this.properties = properties;

        this.parser = Parser.builder().build();
        this.renderer = HtmlRenderer.builder().build();
    }

    public void getAllNotes(Model model) {
        List<MiniNote> notes = miniNotesRepository.findAll();
        model.addAttribute("mininotes", notes);
    }

    @SneakyThrows
    public void uploadImage(MultipartFile file, String description, Model model) {
        File uploadsDir = new File(properties.getUploadDir());
        if (!uploadsDir.exists()) {
            uploadsDir.mkdir();
        }
        String fileId = UUID.randomUUID().toString() + "." +
                file.getOriginalFilename().split("\\.")[1];
        file.transferTo(new File(properties.getUploadDir() + fileId));
        model.addAttribute("description",
                description + " ![](/uploads/" + fileId + ")");
    }

    public void saveMiniNote(String description, Model model) {
        if (description != null && !description.trim().isEmpty()) {
            //translate markup to HTML
            Node document = parser.parse(description.trim());
            String html = renderer.render(document);
            miniNotesRepository.save(new MiniNote( UUID.randomUUID().toString(), html));
            //clean up the textarea
            model.addAttribute("description", "");
        }
    }

    @Override
    public void deleteMiniNote(String noteId, Model model) {
        Optional<MiniNote> note = miniNotesRepository.findById(noteId);
        note.ifPresent(miniNotesRepository::delete);
    }
}
