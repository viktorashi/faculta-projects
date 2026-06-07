CREATE DATABASE IF NOT EXISTS `satelites`;
USE `satelites`;

CREATE TABLE IF NOT EXISTS `satelite` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `name` TEXT NOT NULL,
  `semimajor_axis` DOUBLE NOT NULL CHECK (`semimajor_axis` > 6378),
  `eccentricity` DOUBLE NOT NULL CHECK (`eccentricity` >= 0.0 AND `eccentricity` < 1.0),
  `inclination` DOUBLE NOT NULL CHECK (`inclination` >= 0.0 AND `inclination` <= 180.0),
  `longitude_ascending_node` DOUBLE NOT NULL CHECK (`longitude_ascending_node` >= 0.0 AND `longitude_ascending_node` <= 360.0),
  `argument_of_perigee` DOUBLE NOT NULL CHECK (`argument_of_perigee` >= 0.0 AND `argument_of_perigee` <= 360.0),
  PRIMARY KEY (`id`)
) ENGINE = InnoDB;

-- Seed initial values if they do not exist
INSERT INTO `satelite` (`id`, `name`, `semimajor_axis`, `eccentricity`, `inclination`, `longitude_ascending_node`, `argument_of_perigee`)
SELECT 1, 'fratzica', 6656, 0.9999, 10, 0, 120
WHERE NOT EXISTS (SELECT 1 FROM `satelite` WHERE `id` = 1);

INSERT INTO `satelite` (`id`, `name`, `semimajor_axis`, `eccentricity`, `inclination`, `longitude_ascending_node`, `argument_of_perigee`)
SELECT 2, 'fratzica2', 26560, 0, 55, 0, 120.3
WHERE NOT EXISTS (SELECT 1 FROM `satelite` WHERE `id` = 2);

INSERT INTO `satelite` (`id`, `name`, `semimajor_axis`, `eccentricity`, `inclination`, `longitude_ascending_node`, `argument_of_perigee`)
SELECT 3, 'ISS (International Space Station)', 6790, 0.0005, 51.64, 125, 240
WHERE NOT EXISTS (SELECT 1 FROM `satelite` WHERE `id` = 3);

INSERT INTO `satelite` (`id`, `name`, `semimajor_axis`, `eccentricity`, `inclination`, `longitude_ascending_node`, `argument_of_perigee`)
SELECT 4, 'Hubble Space Telescope', 6918, 0.0003, 28.47, 80, 110
WHERE NOT EXISTS (SELECT 1 FROM `satelite` WHERE `id` = 4);

INSERT INTO `satelite` (`id`, `name`, `semimajor_axis`, `eccentricity`, `inclination`, `longitude_ascending_node`, `argument_of_perigee`)
SELECT 5, 'GEO-Comm 1', 42164, 0.0001, 0.05, 180, 0
WHERE NOT EXISTS (SELECT 1 FROM `satelite` WHERE `id` = 5);

CREATE TABLE IF NOT EXISTS `user` (
  `id` INT NOT NULL AUTO_INCREMENT,
  `email` VARCHAR(255) NOT NULL UNIQUE,
  `username` VARCHAR(255) NOT NULL UNIQUE,
  `password_hash` VARCHAR(255) NOT NULL,
  `reset_token` VARCHAR(255) DEFAULT NULL,
  `reset_token_expiry` DATETIME DEFAULT NULL,
  PRIMARY KEY (`id`)
) ENGINE = InnoDB;
