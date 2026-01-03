package com.java.philly.jud.mininote.controller;

import com.java.philly.jud.mininote.service.IMiniNoteService;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Controller;
import org.springframework.ui.Model;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.multipart.MultipartFile;

@Controller
public class MiniNoteController {

    @Autowired
    private IMiniNoteService miniNoteService;

    @GetMapping("/")
    public String index(Model model) {
        miniNoteService.getAllNotes(model);
        return "index";
    }

    @PostMapping("/note")
    public String saveMiniNotes(@RequestParam("image") MultipartFile file,
                            @RequestParam String description,
                            @RequestParam(required = false) String publish,
                            @RequestParam(required = false) String upload,
                            Model model) throws Exception {

        if (publish != null && publish.equals("Publish")) {
            miniNoteService.saveMiniNote(description, model);
            miniNoteService.getAllNotes(model);
            return "redirect:/";
        }
        if (upload != null && upload.equals("Upload")) {
            if (file != null && file.getOriginalFilename() != null &&
                    !file.getOriginalFilename().isEmpty()) {
                miniNoteService.uploadImage(file, description, model);
            }
            miniNoteService.getAllNotes(model);
            return "index";
        }
        return "index";
    }

    @PostMapping("/delete")
    public String deleteMiniNote(@RequestParam String mininoteid,
                                 Model model) {
        miniNoteService.deleteMiniNote(mininoteid, model);
        miniNoteService.getAllNotes(model);
        return "index";
    }
}